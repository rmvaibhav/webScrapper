package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackdanger/collectlinks"
)

var mu = &sync.Mutex{}

func main() {
	flag.Parse()
	args := flag.Args()
	fmt.Println(args)
	if len(args) < 1 {
		fmt.Println("Please specify base URL to crawl ")
		os.Exit(1)
	}
	_, err := url.ParseRequestURI(args[0])
	if err != nil {
		fmt.Println("Please enter a valid URL to crawl")
		os.Exit(2)
	}

	// Create output file with website name and timestamp
	outputFileName := getOutputFileName(args[0])
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		os.Exit(3)
	}
	defer outputFile.Close()

	queue := make(chan string)
	filteredQueue := make(chan string)

	go func() { queue <- args[0] }()
	go filterQueue(queue, filteredQueue)

	done := make(chan bool)

	for i := 0; i < 5; i++ {
		go func() {
			for uri := range filteredQueue {
				addToQueue(uri, queue, outputFile)
			}
			done <- true
		}()
	}
	<-done
}

func filterQueue(in chan string, out chan string) {
	var seen = make(map[string]bool)
	for val := range in {
		if !seen[val] {
			seen[val] = true
			out <- val
		}
	}
}

func addToQueue(uri string, queue chan string, outputFile *os.File) {
	start := time.Now()
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := http.Client{Transport: transport}
	resp, err := client.Get(uri)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	links := collectlinks.All(resp.Body)
	foundUrls := []string{}
	for _, link := range links {
		absolute := cleanUrl(link, uri)
		foundUrls = append(foundUrls, absolute)
		if uri != "" {
			go func() { queue <- absolute }()
		}
	}
	stop := time.Now()
	display(uri, foundUrls, start, stop, outputFile)
}

func display(uri string, found []string, start time.Time, stop time.Time, outputFile *os.File) {
	mu.Lock()
	fmt.Println("Start time of crawl of this URL:", start)
	fmt.Println("Stop time of crawl of this URL:", stop)
	fmt.Println(uri)

	fmt.Fprintln(outputFile, "Start time of crawl of this URL:", start)
	fmt.Fprintln(outputFile, "Stop time of crawl of this URL:", stop)
	fmt.Fprintln(outputFile, uri)

	for _, str := range found {
		str, err := url.Parse(str)
		if err == nil {
			if str.Scheme == "http" || str.Scheme == "https" {
				fmt.Fprintln(outputFile, "\t", str)
			}
		}
	}
	mu.Unlock()
}

func cleanUrl(href, base string) string {
	uri, err := url.Parse(href)
	if err != nil {
		return ""
	}
	baseUrl, err := url.Parse(base)
	if err != nil {
		return ""
	}
	uri = baseUrl.ResolveReference(uri)
	return uri.String()
}

func getOutputFileName(baseURL string) string {
	baseURL = strings.TrimPrefix(baseURL, "http://")
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.ReplaceAll(baseURL, "/", "-")
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return fmt.Sprintf("%s_%s.txt", baseURL, timestamp)
}

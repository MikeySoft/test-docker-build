package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func run(url string, timeout time.Duration, expected int, client *http.Client) int {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: build request error: %v\n", err)
		return 2
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: request error: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != expected {
		fmt.Fprintf(os.Stderr, "healthcheck: unexpected status: %d (want %d)\n", resp.StatusCode, expected)
		return 1
	}

	return 0
}

func main() {
	url := flag.String("url", "http://127.0.0.1:8081/health", "URL to check")
	timeout := flag.Duration("timeout", 3*time.Second, "HTTP timeout")
	expected := flag.Int("expect", 200, "Expected HTTP status code")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	code := run(*url, *timeout, *expected, client)
	os.Exit(code)
}

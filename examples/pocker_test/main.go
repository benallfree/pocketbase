package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateRandomSubdomain(length int, r *rand.Rand) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[r.Intn(len(charset))]
	}
	return string(result)
}

func makeRequest(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("URL: %s, Error: %v, Duration: %v\n", url, err, duration)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("URL: %s, Status: %d, Duration: %v\n", url, resp.StatusCode, duration)
}

func main() {
	maxRequests := flag.Int("max", 500, "number of different subdomains to hit")
	concurrency := flag.Int("concurrent", 50, "number of concurrent requests")
	flag.Parse()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Pre-generate the list of subdomains
	subdomains := make([]string, *maxRequests)
	for i := 0; i < *maxRequests; i++ {
		subdomains[i] = generateRandomSubdomain(8, r)
	}

	// Create a channel to control concurrency
	semaphore := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	// Setup signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create a done channel to signal goroutines to stop
	done := make(chan struct{})

	// Start the request loop in a separate goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				subdomain := subdomains[r.Intn(len(subdomains))]
				url := fmt.Sprintf("http://%s.lvh.me/api/health", subdomain)

				wg.Add(1)
				semaphore <- struct{}{} // Acquire semaphore

				go func(url string) {
					makeRequest(url, &wg)
					<-semaphore // Release semaphore
				}(url)
			}
		}
	}()

	// Wait for interrupt signal
	<-stop
	fmt.Println("\nShutting down gracefully...")
	close(done)
	wg.Wait()
}

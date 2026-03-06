package main

import (
	"fmt"
	"net/http"
	"sync"
)

func main() {
	const totalRequests = 120
	const url = "http://localhost:8080/healthz"

	var wg sync.WaitGroup
	results := make(chan int, totalRequests)

	fmt.Printf("Sending %d requests to %s...\n", totalRequests, url)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			results <- resp.StatusCode
			resp.Body.Close()
		}()
	}

	wg.Wait()
	close(results)

	counts := make(map[int]int)
	for status := range results {
		counts[status]++
	}

	fmt.Println("\nResults:")
	for status, count := range counts {
		fmt.Printf("Status %d: %d requests\n", status, count)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	query := `query { resources { totalCount } }`
	body := map[string]string{"query": query}
	jsonBody, _ := json.Marshal(body)
	
	fmt.Println("Sending query to port 8081...")
	resp, err := http.Post("http://localhost:8081/query", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("\nRESPONSE FROM SERVER: %s\n", string(b))
}

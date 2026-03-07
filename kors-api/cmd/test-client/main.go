package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func sendRequest(query string, token string) string {
	body := map[string]string{"query": query}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "http://localhost:8080/query", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func main() {
	query := `mutation { createResource(input: { typeName: "tool", initialState: "idle" }) { success error { message } } }`

	// Test 1: No token
	fmt.Println("Test 1: Request WITHOUT token...")
	fmt.Printf("Response: %s\n", sendRequest(query, ""))

	// Test 2: System token
	fmt.Println("\nTest 2: Request WITH 'system' token...")
	fmt.Printf("Response: %s\n", sendRequest(query, "system"))
}

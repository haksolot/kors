package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	query := `
	mutation {
	  registerResourceType(input: {
		name: "tool",
		description: "A manufacturing tool",
		jsonSchema: { type: "object" },
		transitions: { idle: ["in_use"] }
	  }) {
		success
		resourceType {
		  id
		  name
		}
		error {
		  code
		  message
		}
	  }
	}`

	body := map[string]string{
		"query": query,
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post("http://localhost:8080/query", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(respBody))
}

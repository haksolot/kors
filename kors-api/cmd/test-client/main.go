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
	if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return err.Error() }
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func main() {
	fmt.Println("Test: Creating resource while NATS is dead...")
	mutation := `mutation { createResource(input: { typeName: "tool", initialState: "idle" }) { success error { message } } }`
	fmt.Println(sendRequest(mutation, "system"))
}

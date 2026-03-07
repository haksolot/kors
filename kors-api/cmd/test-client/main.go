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
	if err != nil { return fmt.Sprintf("Error: %v", err) }
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func main() {
	fmt.Println("Provisioning module 'tms'...")
	mutation := `mutation { provisionModule(moduleName: "tms") { success moduleName schema username password error { message } } }`
	fmt.Println(sendRequest(mutation, "system"))
}

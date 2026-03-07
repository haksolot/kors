package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	query := `mutation { registerResourceType(input: { name: "tool_trace_test", jsonSchema: {}, transitions: { idle: ["in_use"] } }) { success } }`
	body := map[string]string{"query": query}
	jb, _ := json.Marshal(body)
	
	fmt.Println("Sending traced request...")
	resp, _ := http.Post("http://localhost:8080/query", "application/json", bytes.NewBuffer(jb))
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(b))
}

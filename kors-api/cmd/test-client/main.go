package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func sendRequest(query string) string {
	body := map[string]string{"query": query}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post("http://localhost:8080/query", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func main() {
	fmt.Println("Step 1: Registering type tool_v_final_realtime...")
	regResp := sendRequest(`mutation { registerResourceType(input: { name: "tool_v_final_realtime", description: "Realtime", jsonSchema: {}, transitions: { idle: ["in_use"] } }) { success error { message } } }`)
	fmt.Printf("Register: %s\n", regResp)

	fmt.Println("\nStep 2: Triggering event via resource creation...")
	mutation := `mutation { createResource(input: { typeName: "tool_v_final_realtime", initialState: "idle", metadata: { test: "subscription" } }) { success error { message } } }`
	fmt.Printf("Create: %s\n", sendRequest(mutation))
}

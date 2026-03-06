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
	// 1. Enregistrer le type tool_v6
	fmt.Println("Step 1: Registering type tool_v6...")
	regResp := sendRequest(`mutation { registerResourceType(input: { name: "tool_v6", description: "V6", jsonSchema: {}, transitions: { idle: ["in_use"] } }) { success error { message } } }`)
	fmt.Printf("Register Response: %s\n", regResp)

	// 2. Créer 3 ressources
	fmt.Println("\nStep 2: Creating 3 resources...")
	for i := 1; i <= 3; i++ {
		mutation := fmt.Sprintf(`mutation { createResource(input: { typeName: "tool_v6", initialState: "idle", metadata: { index: %d } }) { success error { message } } }`, i)
		fmt.Printf("Create %d: %s\n", i, sendRequest(mutation))
	}

	// 3. Lister
	fmt.Println("\nStep 3: Fetching...")
	query := `query { resources(first: 2, typeName: "tool_v6") { totalCount edges { node { id } cursor } pageInfo { hasNextPage endCursor } } }`
	fmt.Printf("List Response: %s\n", sendRequest(query))
}

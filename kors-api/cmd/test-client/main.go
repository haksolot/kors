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
	// 1. Enregistrer le type tool_v2
	fmt.Println("Registering resource type tool_v2...")
	registerMutation := `
	mutation {
	  registerResourceType(input: {
		name: "tool_v3",
		description: "A manufacturing tool V3",
		jsonSchema: { type: "object" },
		transitions: { idle: ["in_use"] }
	  }) {
		success
		resourceType { id name }
	  }
	}`
	fmt.Println(sendRequest(registerMutation))

	// 2. Créer une ressource "tool_v3"
	fmt.Println("\nCreating resource...")
	createMutation := `
	mutation {
	  createResource(input: {
		typeName: "tool_v3",
		initialState: "idle",
		metadata: { serial: "SN-999" }
	  }) {
		success
		resource { id state }
		error { message }
	  }
	}`
	createResp := sendRequest(createMutation)
	fmt.Printf("Create Response: %s\n", createResp)

	// Extraire l'ID
	var resData struct {
		Data struct {
			CreateResource struct {
				Resource struct {
					ID string `json:"id"`
				} `json:"resource"`
			} `json:"createResource"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(createResp), &resData)
	resID := resData.Data.CreateResource.Resource.ID

	if resID == "" {
		fmt.Println("Failed to get resource ID.")
		return
	}

	// 3. Faire une transition vers "in_use"
	fmt.Printf("\nTransitioning resource %s to 'in_use'...\n", resID)
	transitionMutation := fmt.Sprintf(`
	mutation {
	  transitionResource(input: {
		resourceId: "%s",
		toState: "in_use",
		metadata: { operator: "User2" }
	  }) {
		success
		resource { id state }
		error { message }
	  }
	}`, resID)
	fmt.Println(sendRequest(transitionMutation))
}

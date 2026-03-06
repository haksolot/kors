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
	// 1. Enregistrer le type tool_v5
	fmt.Println("Registering resource type tool_v5...")
	registerMutation := `
	mutation {
	  registerResourceType(input: {
		name: "tool_v5",
		description: "A manufacturing tool V5 with MinIO",
		jsonSchema: { type: "object" },
		transitions: { idle: ["in_use"] }
	  }) {
		success
		resourceType { id name }
	  }
	}`
	sendRequest(registerMutation)

	// 2. Créer une ressource "tool_v5"
	fmt.Println("Creating resource...")
	createMutation := `
	mutation {
	  createResource(input: {
		typeName: "tool_v5",
		initialState: "idle",
		metadata: { serial: "MINIO-TEST-001" }
	  }) {
		success
		resource { id state }
	  }
	}`
	createResp := sendRequest(createMutation)
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
		fmt.Println("Failed to create resource.")
		return
	}

	// 3. Créer une REVISION avec un fichier fictif
	fmt.Printf("Creating revision for resource %s with file...\n", resID)
	// JVBERi0xLjQKJ... is "PDF-1.4" in base64
	fileBase64 := "JVBERi0xLjQKJWRvY3VtZW50CjEgMCBvYmoKPDwKL1R5cGUgL0NhdGFsb2cKL1BhZ2VzIDIgMCBSCj4+CmVuZG9iagoyIDAgb2JqCjw8Ci9UeXBlIC9QYWdlcwovS2lkcyBbMyAwIFJdCi9Db3VudCAxCj4+CmVuZG9iagozIDAgb2JqCjw8Ci9UeXBlIC9QYWdlCi9QYXJlbnQgMiAwIFIKL01lZGlhQm94IFswIDAgNjEyIDc5Ml0KL0NvbnRlbnRzIDQgMCBSCj4+CmVuZG9iago0IDAgb2JqCjw8Ci9MZW5ndGggNDQKPj4Kc3RyZWFtCkJUCi9GMSAxMiBUZgo3MiA3MjAgVGQKKEhlbGxvIEtPUlMpIFRqCkVUCmVuZHN0cmVhbQplbmRvYmoKeHJlZgowIDUKMDAwMDAwMDAwMCA2NTUzNSBmIAowMDAwMDAwMDE1IDAwMDAwIG4gCjAwMDAwMDAwNjggMDAwMDAgbiAKMDAwMDAwMDEyNSAwMDAwMCBuIAowMDAwMDAwMjMwIDAwMDAwIG4gCnRyYWlsZXIKPDwKL1NpemUgNQovUm9vdCAxIDAgUgo+PgpzdGFydHhyZWYKMzI0CiUlRU9G"
	
	revisionMutation := fmt.Sprintf(`
	mutation {
	  createRevision(input: {
		resourceId: "%s",
		fileName: "plan_maintenance.pdf",
		fileContent: "%s"
	  }) {
		success
		revision { id snapshot filePath }
		error { message }
	  }
	}`, resID, fileBase64)
	
	fmt.Println(sendRequest(revisionMutation))
}

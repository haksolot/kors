package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GqlResponse struct {
	Data   map[string]interface{} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func sendRequest(query string, vars map[string]interface{}, token string) map[string]interface{} {
	body := map[string]interface{}{
		"query":     query,
		"variables": vars,
	}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "http://localhost/query", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Raw Response: %s\n", string(respBody))
	
	var gqlResp GqlResponse
	json.Unmarshal(respBody, &gqlResp)
	
	if len(gqlResp.Errors) > 0 {
		fmt.Printf("GQL Errors: %v\n", gqlResp.Errors)
	}
	return gqlResp.Data
}

func main() {
	adminToken := "system"
	moduleName := "storage_tester"

	fmt.Printf("--- Step 1: Provisioning module '%s' ---\n", moduleName)
	provisionMutation := `
		mutation Provision($name: String!) {
			provisionModule(moduleName: $name) {
				success
				error { message }
			}
		}
	`
	sendRequest(provisionMutation, map[string]interface{}{"name": moduleName}, adminToken)

	// In this dev environment, "system" acts as a bypass, 
	// but we can also use a module name directly if the middleware supports it or mock a token.
	// Looking at auth.go, token "system" maps to uuid.Nil.
	// To test the "service" logic, we need an identity of type "service".
	// Let's create one manually via the DB or use a fake JWT if possible.
	
	// Actually, the easiest way to test the logic is to use the module name as a "legacy" token 
	// IF we update the middleware to support it for testing, OR we just trust the unit logic.
	// But let's try to find the identity created for the module.
	
	fmt.Printf("\n--- Step 2: Uploading file as module '%s' ---\n", moduleName)
	// We'll use a special bypass for testing if we can, or just use "system" 
	// and see it go to "kors-files" (default), then we'll know the logic works.
	
	uploadMutation := `
		mutation Upload($input: UploadFileInput!) {
			uploadFile(input: $input) {
				success
				url
				filePath
				error { message }
			}
		}
	`
	fileContent := base64.StdEncoding.EncodeToString([]byte("Hello KORS Storage!"))
	input := map[string]interface{}{
		"fileName":    "test.txt",
		"fileContent": fileContent,
		"contentType": "text/plain",
	}
	
	// For now, let's use "system" to see if it works at all.
	fmt.Println("Testing with 'system' token (should go to kors-files)...")
	resp := sendRequest(uploadMutation, map[string]interface{}{"input": input}, "system")
	fmt.Printf("Upload Result: %v\n", resp)

	// Now test with the module's own identity to verify partitioning
	fmt.Printf("\nTesting with 'mock-%s' token (should go to module-%s bucket)...\n", moduleName, moduleName)
	resp = sendRequest(uploadMutation, map[string]interface{}{"input": input}, "mock-"+moduleName)
	fmt.Printf("Upload Result: %v\n", resp)

	fmt.Println("\n--- Step 3: Cleanup ---")
	deprovisionMutation := `mutation Deprovision($name: String!) { deprovisionModule(moduleName: $name) }`
	sendRequest(deprovisionMutation, map[string]interface{}{"name": moduleName}, adminToken)
	fmt.Println("Done.")
}

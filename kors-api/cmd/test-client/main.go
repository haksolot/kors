package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GqlResponse struct {
	Data   map[string]interface{} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func sendRequest(query string, token string) string {
	body := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "http://localhost/query", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"errors":[{"message":"%v"}]}`, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody)
}

func main() {
	token := "system"

	// 1. Lister les modules au démarrage
	fmt.Println("Step 1: Listing existing modules...")
	listQuery := `query { provisionedModules }`
	fmt.Printf("List: %s\n", sendRequest(listQuery, token))

	// 2. Provisionner un module temporaire
	moduleName := "test_cleanup"
	fmt.Printf("\nStep 2: Provisioning module '%s'...\n", moduleName)
	provisionMutation := fmt.Sprintf(`mutation { provisionModule(moduleName: "%s") { success username schema } }`, moduleName)
	fmt.Printf("Provision: %s\n", sendRequest(provisionMutation, token))

	// 3. Vérifier sa présence
	fmt.Println("\nStep 3: Verifying module presence in list...")
	resp := sendRequest(listQuery, token)
	if strings.Contains(resp, moduleName) {
		fmt.Printf("SUCCESS: Module '%s' is present.\n", moduleName)
	} else {
		fmt.Printf("FAILURE: Module '%s' NOT found in %s\n", moduleName, resp)
	}

	// 4. Déprovisionner le module
	fmt.Printf("\nStep 4: Deprovisioning module '%s'...\n", moduleName)
	deprovisionMutation := fmt.Sprintf(`mutation { deprovisionModule(moduleName: "%s") }`, moduleName)
	fmt.Printf("Deprovision: %s\n", sendRequest(deprovisionMutation, token))

	// 5. Vérifier sa disparition
	fmt.Println("\nStep 5: Verifying module removal...")
	resp = sendRequest(listQuery, token)
	if !strings.Contains(resp, moduleName) {
		fmt.Printf("SUCCESS: Module '%s' has been removed.\n", moduleName)
	} else {
		fmt.Printf("FAILURE: Module '%s' is STILL present in %s\n", moduleName, resp)
	}
}

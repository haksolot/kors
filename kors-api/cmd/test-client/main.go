package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
	// 1. Calculer une date d'expiration (+10 secondes)
	expiry := time.Now().Add(10 * time.Second).Format(time.RFC3339)
	
	fmt.Printf("Step 1: Granting temporary permission (expires at %s)...\n", expiry)
	
	mutation := fmt.Sprintf(`
	mutation {
	  grantPermission(input: {
		identityId: "00000000-0000-0000-0000-000000000000",
		action: "temporary_test",
		expiresAt: "%s"
	  }) {
		success
		permission { id expiresAt }
	  }
	}`, expiry)
	
	fmt.Println(sendRequest(mutation))
}

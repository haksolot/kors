package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	query := `query { _service { sdl } }`
	body := map[string]string{"query": query}
	jb, _ := json.Marshal(body)
	
	fmt.Println("Checking Apollo Federation support...")
	resp, err := http.Post("http://localhost:8080/query", "application/json", bytes.NewBuffer(jb))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	s := string(b)
	
	if strings.Contains(s, "@key") {
		fmt.Println("\nSUCCESS: Apollo Federation is ACTIVE.")
		fmt.Println("Found @key directives in SDL.")
	} else {
		fmt.Printf("\nFAILURE: Federation might not be active.\nResponse: %s\n", s)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	query := `mutation { deprovisionModule(moduleName: "tms") }`
	body := map[string]string{"query": query}
	jb, _ := json.Marshal(body)
	
	fmt.Println("KORS: Requesting deprovisioning of module 'tms'...")
	req, _ := http.NewRequest("POST", "http://localhost/query", bytes.NewBuffer(jb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer system")

	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(b))
}

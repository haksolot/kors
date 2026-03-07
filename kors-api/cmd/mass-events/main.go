package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	for i := 1; i <= 6; i++ {
		name := fmt.Sprintf("mass_v5_%d", i)
		fmt.Printf("Step %d: Triggering event for %s...\n", i, name)
		
		reg := `mutation { registerResourceType(input: { name: "`+name+`", jsonSchema: {}, transitions: { idle: ["in_use"] } }) { success error { message } } }`
		create := `mutation { createResource(input: { typeName: "`+name+`", initialState: "idle" }) { success error { message } } }`
		
		fmt.Printf("  Reg: %s\n", send(reg))
		fmt.Printf("  Create: %s\n", send(create))
		
		time.Sleep(500 * time.Millisecond)
	}
}

func send(query string) string {
	body := map[string]string{"query": query}
	jb, _ := json.Marshal(body)
	
	req, _ := http.NewRequest("POST", "http://localhost:8080/query", bytes.NewBuffer(jb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer system")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return err.Error() }
	defer resp.Body.Close()
	
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

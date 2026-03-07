package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	query := `
	query($representations: [_Any!]!) {
	  _entities(representations: $representations) {
		... on Resource {
		  id
		  diameter
		  length
		}
	  }
	}`

	variables := map[string]interface{}{
		"representations": []map[string]interface{}{
			{
				"__typename": "Resource",
				"id":         "64f11032-86ac-421c-942b-488707e06a65",
			},
		},
	}

	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	jb, _ := json.Marshal(body)

	fmt.Println("Simulating Gateway call to TMS Subgraph...")
	resp, _ := http.Post("http://localhost:8081/query", "application/json", bytes.NewBuffer(jb))
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(b))
}

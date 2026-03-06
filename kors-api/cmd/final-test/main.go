package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 1. Start WS connection
	c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/query", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	c.WriteJSON(map[string]interface{}{"type": "connection_init"})
	c.WriteJSON(map[string]interface{}{
		"id":   "1",
		"type": "start",
		"payload": map[string]string{
			"query": "subscription { eventWasPublished { type payload } }",
		},
	})

	fmt.Println("WebSocket Subscribed. Triggering event in 2s...")

	// 2. Trigger Event in background after 2s
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("Triggering event now...")
		mutation := `mutation { createResource(input: { typeName: "tool_v_final_realtime", initialState: "idle", metadata: { final: "test" } }) { success } }`
		body := map[string]string{"query": mutation}
		jb, _ := json.Marshal(body)
		http.Post("http://localhost:8080/query", "application/json", bytes.NewBuffer(jb))
	}()

	// 3. Wait for message
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}
		
		// If message is our event
		if bytes.Contains(message, []byte("eventWasPublished")) {
			fmt.Printf("EVENT RECEIVED IN REAL-TIME: %s\n", string(message))
			return // Success!
		}
		fmt.Printf("Received: %s\n", string(message))
	}
}

package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/query"}
	fmt.Printf("Connecting to %s...\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	// 1. Initialize GraphQL-WS protocol
	initMsg := map[string]interface{}{
		"type": "connection_init",
	}
	c.WriteJSON(initMsg)

	// 2. Start Subscription
	subMsg := map[string]interface{}{
		"id":   "1",
		"type": "start",
		"payload": map[string]string{
			"query": "subscription { eventWasPublished { id type payload } }",
		},
	}
	c.WriteJSON(subMsg)

	fmt.Println("Subscribed to eventWasPublished. Waiting for events...")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			fmt.Printf("Event Received: %s\n", message)
		}
	}()

	// Run for 30 seconds or until interrupt
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Closing monitor (timeout).")
	case <-interrupt:
		fmt.Println("Interrupting...")
	}
}

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	// S'abonner à tous les événements KORS
	sub, err := js.SubscribeSync("kors.>")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Listening for events on NATS KORS stream...")
	for {
		msg, err := sub.NextMsg(time.Second * 5)
		if err != nil {
			if err == nats.ErrTimeout {
				fmt.Println("No more events found (timeout).")
				break
			}
			log.Fatal(err)
		}
		fmt.Printf("Message Received [%s]: %s\n", msg.Subject, string(msg.Data))
		msg.Ack()
	}
}

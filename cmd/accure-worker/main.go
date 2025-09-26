package main

import (
	"demo-fees/fees" // Import the inventory service
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"log"
)

func main() {
	c, _ := client.Dial(client.Options{})
	defer c.Close()

	// This worker connects to the shared task queue
	w := worker.New(c, "ORDER_FULFILLMENT_QUEUE", worker.Options{})

	// IMPORTANT: This worker ONLY registers the activities it owns.
	w.RegisterActivity(&fees.Activities{})

	log.Println("Starting Inventory Worker...")
	err := w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Inventory Worker failed", err)
	}
}

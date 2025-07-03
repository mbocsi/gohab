package main

import (
	"fmt"
	"log/slog"

	"github.com/mbocsi/gohab/client"
)

func main() {
	slog.Info("Starting client3")

	tcp := client.NewTCPTransport()
	c := client.NewClient("receiver-b", tcp)

	go func() {
		err := c.Start("localhost:8888")
		if err != nil {
			slog.Error("Error when starting client", "error", err.Error())
		}
	}()

	for !c.Connected {
		// busy wait
	}

	msg, err := c.SendQuery("temperature", nil)
	if err != nil {
		slog.Warn("Error when sending request", "error", err.Error())
	}
	fmt.Printf("%v\n", msg)
	fmt.Printf("Payload: %s\n", string(msg.Payload))
}

package main

import (
	"fmt"
	"log/slog"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
)

func handleSensorTemp(msg proto.Message) error {
	switch msg.Type {
	case "status":
		fmt.Printf("Received sensor status: %s\n", string(msg.Payload))
	case "data":
		fmt.Printf("Received temperature: %s\n", string(msg.Payload))
	}
	return nil
}

func main() {
	slog.Info("Starting client2")

	tcp := client.NewTCPTransport()
	c := client.NewClient("receiver-a", tcp)

	c.Subscribe("temperature", handleSensorTemp)

	err := c.Start("localhost:8888")
	if err != nil {
		panic(err)
	}
}

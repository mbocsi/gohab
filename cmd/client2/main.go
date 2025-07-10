package main

import (
	"encoding/json"
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

func handleDisplayCommand(msg proto.Message) error {
	type inputSchema struct {
		Brightness float64 `json:"brightness"`
	}
	command := &inputSchema{}
	err := json.Unmarshal(msg.Payload, command)
	if err != nil {
		return err
	}
	fmt.Printf("Setting brightness to %f\n", command.Brightness)
	return nil
}

func main() {
	slog.Info("Starting client2")

	ws := client.NewWebSocketTransport()
	c := client.NewClient("display-a", ws)

	err := c.AddFeature(proto.Feature{
		Name:        "display_temperature",
		Description: "The temperature field of the display in the living room",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				Description: "Set the color of the temperature field on the display",
				InputSchema: map[string]proto.DataType{
					"brightness": {Type: "number", Range: []float64{0, 100}},
				},
			},
		}},
	)

	c.RegisterCommandHandler("display_temperature", handleDisplayCommand)
	c.Subscribe("temperature", handleSensorTemp)

	err = c.Start("ws://localhost:8889")
	if err != nil {
		panic(err)
	}
}

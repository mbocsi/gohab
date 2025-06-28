package main

import (
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
)

func main() {
	slog.Info("Starting client1")

	tcp := client.NewTCPTransport()
	c := client.NewClient("sensor-a", tcp)

	err := c.AddCapability(proto.Capability{
		Name:        "temperature",
		Description: "A temperature sensor outside on the deck",
		Methods: proto.CapabilityMethods{
			Data: proto.Method{Description: "Temperature outside the deck",
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number"},
					"time":        {Type: "string"}}},
			Status: proto.Method{
				Description: "Whether the sensor works or not",
				OutputSchema: map[string]proto.DataType{
					"status": {Type: "enum", Enum: []string{"ok", "bad"}},
				}},
		},
	},
	)

	dataFn, err := c.GetDataFunction("temperature")
	if err != nil {
		panic(err)
	}

	statusFn, err := c.GetStatusFunction("temperature")
	if err != nil {
		panic(err)
	}

	go c.Start("localhost:8080") // Start in background

	ticker := time.NewTicker(5 * time.Second)
	for temp := 20.0; ; temp += 0.1 {
		<-ticker.C
		dataFn(temp)
		statusFn("ok")
	}
}

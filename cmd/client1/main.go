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
		Name:     "temperature",
		Type:     "sensor",
		Access:   "read",
		DataType: "number",
		Topic: proto.CapabilityTopic{
			Name:  "sensor/temp",
			Types: []string{"data", "status"},
		},
	})
	if err != nil {
		panic(err)
	}

	dataFn, statusFn, err := c.GenerateCapabilityFunctions("temperature", nil, nil)
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

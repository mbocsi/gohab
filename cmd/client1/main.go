package main

import (
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
)

type DataPayload struct {
	Temperature float64 `json:"temperature"`
	Timestamp   string  `json:"timestamp"`
}

type StatusPayload struct {
	Status string `json:"status"`
}

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
					"temperature": {Type: "number", Unit: "Celcius"},
					"timestamp":   {Type: "string"}}},
			Status: proto.Method{
				Description: "Whether the sensor works or not",
				OutputSchema: map[string]proto.DataType{
					"status": {Type: "enum", Enum: []string{"ok", "bad"}}}},
			Query: proto.Method{
				Description: "Get the current temperature",
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Calcius"},
					"timestamp":   {Type: "string"}}},
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

	temp := 20.0
	err = c.RegisterQueryHandler("temperature", func(msg proto.Message) (any, error) {
		return DataPayload{Temperature: temp, Timestamp: time.Now().String()}, nil
	})

	go c.Start("localhost:8080") // Start in background

	ticker := time.NewTicker(5 * time.Second)
	for ; ; temp += 0.1 {
		<-ticker.C
		dataFn(DataPayload{Temperature: temp, Timestamp: time.Now().String()})
		statusFn(StatusPayload{Status: "ok"})
	}
}

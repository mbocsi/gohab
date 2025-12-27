package main

import (
	"log/slog"
	"math/rand"
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

	err := c.AddFeature(proto.Feature{
		Name:        "temperature",
		Description: "A temperature sensor outside on the deck",
		Methods: proto.FeatureMethods{
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
					"temperature": {Type: "number", Unit: "Celcius"},
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

	temp := rand.Float64()*5 + 17
	err = c.RegisterQueryHandler("temperature", func(msg proto.Message) (any, error) {
		return DataPayload{Temperature: temp, Timestamp: time.Now().String()}, nil
	})

	go c.Start("auto") // Auto-discover server via mDNS

	ticker := time.NewTicker(5 * time.Second)
	for {
		<-ticker.C
		temp = rand.Float64()*5 + 17
		dataFn(DataPayload{Temperature: temp, Timestamp: time.Now().String()})
		statusFn(StatusPayload{Status: "ok"})
		brightness := rand.Float64() * 100
		c.SendCommand("display_temperature", struct {
			Brightness float64 `json:"brightness"`
		}{brightness})
	}
}

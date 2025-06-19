package main

import (
	"log/slog"
	"os"

	"github.com/mbocsi/gohab/broker"
	"github.com/mbocsi/gohab/transport"
)

func main() {
	logger := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logger))

	broker := broker.NewBroker()

	tcpServer := transport.NewTCPTransport("0.0.0.0:8080")
	tcpServer.OnMessage(func(msg transport.Message) {
		broker.Publish(msg.Topic, msg.Payload, msg.Sender)
	})
	if err := tcpServer.Start(); err != nil {
		slog.Error("Error starting server", "error", err.Error())
	}
}

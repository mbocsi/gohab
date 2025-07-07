package main

import (
	"log/slog"

	"github.com/mbocsi/gohab/server"
)

func main() {
	// Create dependencies
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	
	// Create TCP transport
	tcpServer := server.NewTCPTransport("0.0.0.0:8888")
	tcpServer.SetName("Main TCP server")
	tcpServer.SetMaxClients(2)
	tcpServer.SetDescription("The main TCP server for just the display and temperature")
	
	// Create server with dependencies
	gohabServer := server.NewGohabServer(registry, broker)
	gohabServer.RegisterTransport(tcpServer)

	if err := gohabServer.Start(":8080"); err != nil {
		slog.Error("Error starting gohab server", "error", err.Error())
	}
}

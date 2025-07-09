package main

import (
	"log/slog"

	"github.com/mbocsi/gohab/server"
	"github.com/mbocsi/gohab/services"
	"github.com/mbocsi/gohab/web"
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

	// Create in-memory transport
	inMemoryTransport := web.NewInMemoryTransport()
	gohabServer.RegisterTransport(inMemoryTransport)

	// Create service layer (Maybe should just take Gohabserver as dependency)
	serviceManager := services.NewServiceManager(
		gohabServer.GetRegistry(),
		gohabServer.GetBroker(),
		gohabServer.GetTransports(),
		gohabServer.GetTopicSources,
		gohabServer.Handle,
	)

	// Create web client with service layer and transport
	webClient := web.NewWebClient(serviceManager.GetServices())
	inMemoryTransport.RegisterClient(webClient)

	go webClient.Start(":8080")
	defer webClient.Shutdown()

	if err := gohabServer.Start(); err != nil {
		slog.Error("Error starting gohab server", "error", err.Error())
	}
}

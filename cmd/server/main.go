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

	// Create WebSocket transport
	wsServer := server.NewWSTransport("0.0.0.0:8889")
	wsServer.SetName("Main WebSocket server")
	wsServer.SetMaxClients(2)
	wsServer.SetDescription("The main WebSocket server for web clients")

	// Create server with dependencies and optional logging config
	gohabServer := server.NewGohabServer(registry, broker)

	// Example: Set custom logging configuration
	// gohabServer.SetLogConfig(server.QuietLogConfig())     // Only errors
	// gohabServer.SetLogConfig(server.SuppressedLogConfig()) // No logs at all
	// gohabServer.SetLogConfig(server.DefaultLogConfig())   // Default debug level

	gohabServer.RegisterTransport(tcpServer)
	gohabServer.RegisterTransport(wsServer)

	// Create LoRa transport (using mock hardware for development)
	// For real hardware, replace with your actual HardwareInterface implementation
	loraConfig := server.DefaultSX1276Config() // 868MHz EU configuration
	// mockHW := &MockHardwareInterface{}  // This would be in a test file
	// mockLoraRadio, err := server.NewSX1276Radio(loraConfig, mockHW)
	// For now, skip LoRa transport in main.go since mock is in test files
	// Uncomment the LoRa transport setup when you have real hardware interface
	_ = loraConfig // Prevent unused variable warning

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

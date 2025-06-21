package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mbocsi/gohab/broker"
	"github.com/mbocsi/gohab/mcp"
	"github.com/mbocsi/gohab/transport"
)

type App struct {
	Registery  *DeviceRegistry
	Broker     *broker.Broker
	MCPServer  mcp.Server
	Transports []transport.Transport
}

func NewApp(registery *DeviceRegistry, broker *broker.Broker, mcpServer mcp.Server) *App {
	return &App{Registery: registery, Broker: broker, MCPServer: mcpServer}
}

func (a *App) Start(ctx context.Context) {
	go a.MCPServer.Run()
	for _, t := range a.Transports {
		go t.Start()
	}

	<-ctx.Done()
	slog.Info("Shutting down transports and server")

	for _, t := range a.Transports {
		t.Shutdown()
	}
}

func (a *App) RegisterTransport(t transport.Transport) {
	t.OnMessage(a.Handle)
	t.OnConnect((a.RegisterDevice))
	t.OnDisconnect(func(client transport.Client) { a.Registery.Delete(client.Meta().Id) })
	a.Transports = append(a.Transports, t)
}

func (a *App) generateDeviceID() string {
	// Option 1: Use the IP address (not stable if NAT is involved)
	// return "device-" + strings.ReplaceAll(client.RemoteAddr(), ":", "_")

	// Option 2: Generate short UUID (preferred)
	return "device-" + uuid.NewString()[:8]
}

func (a *App) RegisterDevice(client transport.Client) error {
	// Generate a real ID and inject it
	realID := a.generateDeviceID()
	client.Meta().Id = realID

	a.Registery.Store(client)

	ackPayload := map[string]string{
		"assigned_id": realID,
		"status":      "ok",
	}

	ackPayloadBytes, err := json.Marshal(ackPayload)
	if err != nil {
		return err
	}

	ack := transport.Message{
		Type:      "identify_ack",
		Payload:   ackPayloadBytes,
		Sender:    "server",
		Timestamp: time.Now().Unix(),
	}
	client.Send(ack)
	return nil
}

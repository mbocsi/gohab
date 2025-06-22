package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mbocsi/gohab/proto"
)

type Coordinator struct {
	Registery  *DeviceRegistry
	Broker     *Broker
	MCPServer  Server
	Transports []Transport
}

func NewCoordinator(registery *DeviceRegistry, broker *Broker, mcpServer Server) *Coordinator {
	return &Coordinator{Registery: registery, Broker: broker, MCPServer: mcpServer}
}

func (c *Coordinator) Start(ctx context.Context) error {
	// TODO: Add context to check if go routines exit for some reason
	go c.MCPServer.Start()
	for _, t := range c.Transports {
		go t.Start()
	}

	<-ctx.Done()
	slog.Info("Shutting down transports and server")

	for _, t := range c.Transports {
		t.Shutdown()
	}
	return nil
}

func (c *Coordinator) RegisterTransport(t Transport) {
	t.OnMessage(c.Handle)
	t.OnConnect((c.RegisterDevice))
	t.OnDisconnect(func(client Client) { c.Registery.Delete(client.Meta().Id) })
	c.Transports = append(c.Transports, t)
}

func (c *Coordinator) generateDeviceID() string {
	// Option 1: Use the IP address (not stable if NAT is involved)
	// return "device-" + strings.ReplaceAll(client.RemoteAddr(), ":", "_")

	// Option 2: Generate short UUID (preferred)
	return "device-" + uuid.NewString()[:8]
}

func (c *Coordinator) RegisterDevice(client Client) error {
	// Generate a real ID and inject it
	realID := c.generateDeviceID()
	client.Meta().Id = realID

	c.Registery.Store(client)

	ackPayload := proto.IdAckPayload{
		AssignedId: realID,
		Status:     "ok",
	}

	ackPayloadBytes, err := json.Marshal(ackPayload)
	if err != nil {
		return err
	}

	ack := proto.Message{
		Type:      "identify_ack",
		Payload:   ackPayloadBytes,
		Sender:    "server",
		Timestamp: time.Now().Unix(),
	}
	client.Send(ack)
	slog.Info("Identified client", "id", realID, "name", client.Meta().Name)
	return nil
}

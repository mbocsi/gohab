package server

import (
	"context"
	"log/slog"
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

func (c *Coordinator) RegisterDevice(client Client) error {
	c.Registery.Store(client)

	// ackPayload := proto.IdAckPayload{
	// 	AssignedId: client.Meta().Id,
	// 	Status:     "ok",
	// }

	// ackPayloadBytes, err := json.Marshal(ackPayload)
	// if err != nil {
	// 	return err
	// }

	// ack := proto.Message{
	// 	Type:      "identify_ack",
	// 	Payload:   ackPayloadBytes,
	// 	Sender:    "server",
	// 	Timestamp: time.Now().Unix(),
	// }
	// client.Send(ack)
	slog.Info("Registered client", "id", client.Meta().Id)
	return nil
}

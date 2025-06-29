package server

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/proto"
)

func (c *Coordinator) Handle(msg proto.Message) {
	switch msg.Type {
	case "identify":
		c.handleIdentify(msg)

	case "data", "status":
		c.handleData(msg)

	case "subscribe", "unsubscribe":
		c.handleSubscription(msg)

	case "command":
		c.handleCommand(msg)

	case "query", "response":
		c.handleQuery(msg)

	default:
		slog.Warn("Unhandled message type", "type", msg.Type, "sender", msg.Sender)
	}
}

// ---------- identify ---------- //

func (c *Coordinator) handleIdentify(msg proto.Message) {
	id := msg.Sender
	client, ok := c.Registery.Get(id)
	if !ok {
		slog.Error("Unknown client sent identify message", "id", id)
		return
	}

	var idPayload proto.IdentifyPayload
	if err := json.Unmarshal(msg.Payload, &idPayload); err != nil {
		slog.Warn("Invalid JSON identify payload received", "id", id, "error", err.Error(), "data", string(msg.Payload))
		return
	}

	newMetadata := DeviceMetadata{
		Id:           id,
		Name:         idPayload.ProposedName,
		LastSeen:     time.Now(),
		Firmware:     idPayload.Firmware,
		Capabilities: idPayload.Capabilities}

	*client.Meta() = newMetadata

	ackPayload := proto.IdAckPayload{
		AssignedId: client.Meta().Id,
		Status:     "ok",
	}

	ackPayloadBytes, err := json.Marshal(ackPayload)
	if err != nil {
		slog.Warn("There was an error marshalling ack payload", "error", err.Error())
		return
	}

	ack := proto.Message{
		Type:      "identify_ack",
		Payload:   ackPayloadBytes,
		Sender:    "server",
		Timestamp: time.Now().Unix(),
	}
	client.Send(ack)
}

// ---------- data / status ---------- //

func (c *Coordinator) handleData(msg proto.Message) {
	// Fan-out to all subscribers of msg.Topic.
	//
	//  ⚠  json.RawMessage *is already* a []byte alias,
	//     so we can pass it straight to Publish.
	c.Broker.Publish(msg)
}

// ---------- stubs for other message kinds ---------- //

func (c *Coordinator) handleSubscription(msg proto.Message) {
	switch msg.Type {
	case "subscribe":
		client, ok := c.Registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub proto.SubscriptionPayload
		json.Unmarshal(msg.Payload, &sub)
		for _, topic := range sub.Topics {
			c.Broker.Subscribe(topic, client)
		}

	case "unsubscribe":
		client, ok := c.Registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub proto.SubscriptionPayload
		json.Unmarshal(msg.Payload, &sub)
		for _, topic := range sub.Topics {
			c.Broker.Unsubscribe(topic, client)
		}
	}
}

// TODO: Implement these
func (c *Coordinator) handleCommand(msg proto.Message) {}
func (c *Coordinator) handleQuery(msg proto.Message)   {}

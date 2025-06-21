package server

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/proto"
)

func (c *Coordinator) Handle(msg proto.Message) {
	switch msg.Type {
	case "data", "status", "info":
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

// ---------- data / status / info ---------- //

func (c *Coordinator) handleData(msg proto.Message) {
	// Fan-out to all subscribers of msg.Topic.
	//
	//  ⚠  json.RawMessage *is already* a []byte alias,
	//     so we can pass it straight to Publish.
	c.Broker.Publish(msg)

	slog.Debug("Data forwarded",
		"topic", msg.Topic,
		"sender", msg.Sender,
		"bytes", len(msg.Payload),
		"ts", time.Unix(msg.Timestamp, 0).UTC(),
	)
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

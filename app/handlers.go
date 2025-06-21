package app

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/transport"
)

func (a *App) Handle(msg transport.Message) {
	switch msg.Type {
	case "data", "status", "info":
		a.handleData(msg)

	case "subscribe", "unsubscribe":
		a.handleSubscription(msg)

	case "command":
		a.handleCommand(msg)

	case "query", "response":
		a.handleQuery(msg)

	default:
		slog.Warn("Unhandled message type", "type", msg.Type, "sender", msg.Sender)
	}
}

// ---------- data / status / info ---------- //

func (a *App) handleData(msg transport.Message) {
	// Fan-out to all subscribers of msg.Topic.
	//
	//  ⚠  json.RawMessage *is already* a []byte alias,
	//     so we can pass it straight to Publish.
	a.Broker.Publish(msg)

	slog.Debug("Data forwarded",
		"topic", msg.Topic,
		"sender", msg.Sender,
		"bytes", len(msg.Payload),
		"ts", time.Unix(msg.Timestamp, 0).UTC(),
	)
}

// ---------- stubs for other message kinds ---------- //

func (a *App) handleSubscription(msg transport.Message) {
	switch msg.Type {
	case "subscribe":
		client, ok := a.Registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub transport.SubscriptionPayload
		json.Unmarshal(msg.Payload, &sub)
		for _, topic := range sub.Topics {
			a.Broker.Subscribe(topic, client)
		}

	case "unsubscribe":
		client, ok := a.Registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub transport.SubscriptionPayload
		json.Unmarshal(msg.Payload, &sub)
		for _, topic := range sub.Topics {
			a.Broker.Unsubscribe(topic, client)
		}
	}
}

// TODO: Implement these
func (a *App) handleCommand(msg transport.Message) {}
func (a *App) handleQuery(msg transport.Message)   {}

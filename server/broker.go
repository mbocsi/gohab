package server

import (
	"log/slog"
	"sync"

	"github.com/mbocsi/gohab/proto"
)

type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[Client]struct{} // Map topic to hashset of Clients
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[Client]struct{}),
	}
}

func (b *Broker) Subscribe(topic string, client Client) {
	slog.Debug("Subscribing", "topic", topic, "clientId", client.Meta().Id)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subs[topic] == nil {
		b.subs[topic] = make(map[Client]struct{})
	}
	b.subs[topic][client] = struct{}{}
}

func (b *Broker) Publish(msg proto.Message) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sentCount := 0
	// Potentially make this non blocking?
	for client := range b.subs[msg.Topic] {
		err := client.Send(msg)
		if err != nil {
			slog.Warn("There was an error publishing a message to a subscriber", "type", msg.Type, "topic", msg.Topic, "error", err.Error())
			continue
		}
		sentCount++
	}
	slog.Debug("Message published",
		"type", msg.Type,
		"topic", msg.Topic,
		"sender", msg.Sender,
		"subscribers", sentCount,
		"size", len(msg.Payload),
	)
}

func (b *Broker) Unsubscribe(topic string, client Client) {
	slog.Debug("Unsubscribing", "topic", topic, "clientId", client.Meta().Id)
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subs[topic]; ok {
		if _, exists := subs[client]; exists {
			delete(subs, client)
		} else {
			slog.Warn("Did not find client in topic to unsubscribe", "topic", topic, "client", client)
		}
		if len(subs) == 0 {
			delete(b.subs, topic)
		}
	}
}

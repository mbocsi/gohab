package broker

import (
	"log/slog"
	"sync"

	"github.com/mbocsi/gohab/transport"
)

type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[transport.Client]struct{} // Map topic to hashset of Clients
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[transport.Client]struct{}),
	}
}

func (b *Broker) Subscribe(topic string, client transport.Client) {
	slog.Debug("Subscribing", "topic", topic, "client", client)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subs[topic] == nil {
		b.subs[topic] = make(map[transport.Client]struct{})
	}
	b.subs[topic][client] = struct{}{}
}

func (b *Broker) Publish(msg transport.Message) {
	slog.Debug("Publishing message", "message", msg)
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Potentially make this non blocking?
	for client := range b.subs[msg.Topic] {
		client.Send(msg)
	}
}

func (b *Broker) Unsubscribe(topic string, client transport.Client) {
	slog.Debug("Unsubscribing", "topic", topic, "client", client)
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

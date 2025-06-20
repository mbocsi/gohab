package broker

import (
	"log/slog"
	"sync"

	"github.com/mbocsi/gohab/transport"
)

type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[chan transport.Message]struct{} // Map topic to hashset of Message channels
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[chan transport.Message]struct{}),
	}
}

func (b *Broker) Subscribe(topic string, ch chan transport.Message) {
	slog.Debug("Subscribing", "topic", topic, "channel", ch)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subs[topic] == nil {
		b.subs[topic] = make(map[chan transport.Message]struct{})
	}
	b.subs[topic][ch] = struct{}{}
}

func (b *Broker) Publish(msg transport.Message) {
	slog.Debug("Publishing message", "message", msg)
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subs[msg.Topic] {
		select {
		case ch <- msg:
		default:
			slog.Error("Dropped message to %v (buffer full)", ch)
		}
	}
}

func (b *Broker) Unsubscribe(topic string, ch chan transport.Message) {
	slog.Debug("Unsubscribing", "topic", topic, "channel", ch)
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subs[topic]; ok {
		if _, exists := subs[ch]; exists {
			delete(subs, ch)
		} else {
			slog.Warn("Did not find channel %v in topic '%s' subs", ch, topic)
		}
		if len(subs) == 0 {
			delete(b.subs, topic)
		}
	}

}

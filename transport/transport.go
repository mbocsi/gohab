package transport

import "encoding/json"

type Message struct {
	Type      string          `json:"type"`      // "command", "status", "info", "query", "response"
	Topic     string          `json:"topic"`     // logical routing (e.g., "light/123/state")
	Sender    string          `json:"sender"`    // sender ID (e.g., device ID or server)
	Recipient string          `json:"recipient"` // optional (for direct device communication)
	Payload   json.RawMessage `json:"payload"`   // raw JSON; allows flexible schema per message type
	Timestamp int64           `json:"timestamp"` // UNIX timestamp in seconds
}

type Transport interface {
	Start() error
	OnMessage(func(Message))
	Shutdown() error
}

type Client interface {
	Send(Message) error
}

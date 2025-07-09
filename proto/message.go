package proto

import (
	"encoding/json"
)

type Message struct {
	Type      string          `json:"type"`                // "command", "status", "info", "query", "response"
	Topic     string          `json:"topic,omitempty"`     // logical routing (e.g., "light/123/state")
	Sender    string          `json:"sender,omitempty"`    // sender ID (e.g., device ID or server)
	Recipient string          `json:"recipient,omitempty"` // optional (for direct device communication)
	Payload   json.RawMessage `json:"payload"`             // raw JSON; allows flexible schema per message type
	Timestamp int64           `json:"timestamp"`           // UNIX timestamp in seconds
}

type IdentifyPayload struct {
	ProposedName string       `json:"proposed_name"` // Optional human-readable alias
	Firmware     string       `json:"firmware"`      // Firmware version or build tag
	Features []Feature `json:"features"`  // List of introspectable features
}

type SubscriptionPayload struct {
	Topics []string `json:"topics"` // e.g. ["sensor/temp", "device/77/status"]
}

type IdAckPayload struct {
	AssignedId string `json:"assigned_id"`
	Status     string `json:"status"`
}

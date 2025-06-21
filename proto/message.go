package proto

import (
	"encoding/json"
)

type Message struct {
	Type      string          `json:"type"`      // "command", "status", "info", "query", "response"
	Topic     string          `json:"topic"`     // logical routing (e.g., "light/123/state")
	Sender    string          `json:"sender"`    // sender ID (e.g., device ID or server)
	Recipient string          `json:"recipient"` // optional (for direct device communication)
	Payload   json.RawMessage `json:"payload"`   // raw JSON; allows flexible schema per message type
	Timestamp int64           `json:"timestamp"` // UNIX timestamp in seconds
}

type IdentifyPayload struct {
	ProposedName string       `json:"proposed_name"` // Optional human-readable alias
	Firmware     string       `json:"firmware"`      // Firmware version or build tag
	Capabilities []Capability `json:"capabilities"`  // List of introspectable capabilities
}

type Capability struct {
	Name        string          `json:"name"`                  // Unique key: "temperature", "led", etc.
	Type        string          `json:"type"`                  // "sensor", "actuator", "config", etc.
	Access      string          `json:"access"`                // "read", "write", "read/write"
	DataType    string          `json:"data_type"`             // "number", "string", "bool", "enum"
	Unit        string          `json:"unit,omitempty"`        // Optional: °C, %, lux, etc.
	Range       []float64       `json:"range,omitempty"`       // Optional: [min, max]
	Enum        []string        `json:"enum,omitempty"`        // For enum data types: ["on", "off"]
	Topic       CapabilityTopic `json:"topic"`                 // Routing topics (may be logical or server-defined)
	Description string          `json:"description,omitempty"` // Human-readable purpose
	LLMTags     []string        `json:"llm_tags,omitempty"`    // Hints for LLMs or indexing
}

type CapabilityTopic struct {
	Publish string `json:"publish,omitempty"` // Topic used by the device to send data
	Command string `json:"command,omitempty"` // Topic the device listens on for commands
	Status  string `json:"status,omitempty"`  // Topic to send status changes
}

type SubscriptionPayload struct {
	Topics []string `json:"topics"` // e.g. ["sensor/temp", "device/77/status"]
}

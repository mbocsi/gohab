package proto

import (
	"errors"
	"fmt"
	"strings"
)

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

func NewCapability() Capability {
	return Capability{}
}

func (c *Capability) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("capability name is required")
	}
	if strings.TrimSpace(c.Type) == "" {
		return errors.New("capability type is required")
	}
	if c.Type != "sensor" && c.Type != "actuator" && c.Type != "config" {
		return fmt.Errorf("invalid capability type: %q", c.Type)
	}
	if strings.TrimSpace(c.Access) == "" {
		return errors.New("capability access is required")
	}
	if c.Access != "read" && c.Access != "write" && c.Access != "read/write" {
		return fmt.Errorf("invalid access mode: %q", c.Access)
	}
	if strings.TrimSpace(c.DataType) == "" {
		return errors.New("capability data_type is required")
	}
	switch c.DataType {
	case "number", "string", "bool":
		// no extra validation
	case "enum":
		if len(c.Enum) == 0 {
			return errors.New("enum capabilities must define Enum field")
		}
	default:
		return fmt.Errorf("invalid data_type: %q", c.DataType)
	}
	if err := c.Topic.Validate(); err != nil {
		return fmt.Errorf("invalid topic: %w", err)
	}
	return nil
}

type CapabilityTopic struct {
	Name  string   `json:"name"`
	Types []string `json:"types,omitempty"`
}

var validTypes = map[string]bool{
	"data":    true,
	"status":  true,
	"command": true,
	"query":   true,
}

func (ct CapabilityTopic) Validate() error {
	if strings.TrimSpace(ct.Name) == "" {
		return errors.New("topic name is required")
	}
	if len(ct.Types) == 0 {
		return errors.New("at least one message type must be specified")
	}
	for _, t := range ct.Types {
		if !validTypes[t] {
			return fmt.Errorf("invalid message type in topic: %q", t)
		}
	}
	return nil
}

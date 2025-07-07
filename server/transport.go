package server

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mbocsi/gohab/proto"
)

type Transport interface {
	Start() error
	OnMessage(func(proto.Message))
	OnConnect(func(Client) error)
	OnDisconnect(func(Client))
	Shutdown() error
	Meta() TransportMetadata
	SetName(name string)
	SetDescription(description string)
}

type TransportMetadata struct {
	Name        string // Human-friendly name, e.g., "TCP Server", "WebSocket Gateway"
	Protocol    string // Protocol name, e.g., "tcp", "websocket", "http"
	Address     string // Bind address, e.g., "0.0.0.0:8080"
	Description string // Optional, short purpose/use case

	Clients    map[string]Client // Current active clients
	MaxClients int               // Max allowed clients (if applicable, else 0)
	Connected  bool              // Whether the transport is currently running/bound
}

type DeviceMetadata struct {
	Id           string
	Name         string
	LastSeen     time.Time
	Firmware     string
	Capabilities map[string]proto.Capability
	Subs         map[string]struct{}
	Transport    Transport
	Mu           sync.RWMutex
}

type Client interface {
	Send(proto.Message) error
	Meta() *DeviceMetadata
}

func generateClientId(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

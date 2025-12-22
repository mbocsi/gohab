package web

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// InMemoryTransport implements the Transport interface for in-memory clients
type InMemoryTransport struct {
	onMessage    func(proto.Message)
	onConnect    func(server.Client) error
	onDisconnect func(server.Client)

	name        string
	description string
	clients     map[string]server.Client
	cmu         sync.RWMutex

	maxClients int
	connected  bool
}

func NewInMemoryTransport() *InMemoryTransport {
	return &InMemoryTransport{
		name:        "In-Memory Transport",
		description: "In-memory transport for web UI clients",
		clients:     make(map[string]server.Client),
		maxClients:  4,
		connected:   false,
	}
}

func (wt *InMemoryTransport) Start() error {
	slog.Info("Starting in-memory transport", "addr", "in-memory")
	if wt.onConnect == nil || wt.onDisconnect == nil || wt.onMessage == nil {
		return fmt.Errorf("The OnConnect, OnDisconnect, or OnMessage function is not defined. This transport is likely being called outside of the server.")
	}
	wt.connected = true
	return nil
}

func (wt *InMemoryTransport) OnMessage(handler func(proto.Message)) {
	wt.onMessage = handler
}

func (wt *InMemoryTransport) OnConnect(handler func(server.Client) error) {
	wt.onConnect = handler
}

func (wt *InMemoryTransport) OnDisconnect(handler func(server.Client)) {
	wt.onDisconnect = handler
}

func (wt *InMemoryTransport) Shutdown() error {
	wt.cmu.Lock()
	defer wt.cmu.Unlock()

	for _, client := range wt.clients {
		if wt.onDisconnect != nil {
			wt.onDisconnect(client)
		}
	}

	wt.clients = make(map[string]server.Client)
	wt.connected = false

	slog.Info("Web transport shut down")
	return nil
}

func (wt *InMemoryTransport) Meta() server.TransportMetadata {
	wt.cmu.RLock()
	clients := wt.clients
	wt.cmu.RUnlock()
	return server.TransportMetadata{
		ID:          "web-transport",
		Name:        wt.name,
		Description: wt.description,
		Protocol:    "memory",
		Address:     "na",
		Clients:     clients,
		MaxClients:  wt.maxClients,
		Connected:   wt.connected,
	}
}

func (wt *InMemoryTransport) SetName(name string) {
	wt.name = name
}

func (wt *InMemoryTransport) SetDescription(description string) {
	wt.description = description
}

// RegisterClient registers any client with the transport
func (wt *InMemoryTransport) RegisterClient(client server.Client) error {
	wt.cmu.Lock()
	defer wt.cmu.Unlock()

	client.Meta().Transport = wt

	wt.clients[client.Meta().Id] = client

	return wt.onConnect(client)
}

// UnregisterClient unregisters a web client from the transport (Unique to this transport)
func (wt *InMemoryTransport) UnregisterClient(clientID string) {
	wt.cmu.Lock()
	client, exists := wt.clients[clientID]
	if exists {
		delete(wt.clients, clientID)
	}
	wt.cmu.Unlock()

	if exists && wt.onDisconnect != nil {
		wt.onDisconnect(client)
	}
}

// SendMessage sends a message through the transport (Unique to this transport, needs sender id)
func (wt *InMemoryTransport) SendMessage(msg proto.Message) error {
	wt.onMessage(msg)
	return nil
}

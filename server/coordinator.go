package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

type Coordinator struct {
	Registery  *DeviceRegistry
	Broker     *Broker
	MCPServer  *MCPServer
	Transports []Transport
	Templates  *Templates

	topicSourcesMu sync.RWMutex
	topicSources   map[string]string // topic → deviceID
}

func NewCoordinator(registery *DeviceRegistry, broker *Broker, mcpServer *MCPServer) *Coordinator {
	return &Coordinator{Registery: registery, Broker: broker, MCPServer: mcpServer, Templates: NewTemplates("templates/*.html"), topicSources: make(map[string]string)}
}

func (c *Coordinator) Start(ctx context.Context, addr string) error {
	// TODO: Add context to check if go routines exit for some reason
	if c.MCPServer != nil {
		go c.MCPServer.Start()
	}
	for _, t := range c.Transports {
		go t.Start()
	}

	server := &http.Server{
		Addr:    addr, // configurable
		Handler: c.Routes(),
	}

	slog.Info("Starting http server", "addr", addr)
	go server.ListenAndServe()

	<-ctx.Done()
	slog.Info("Shutting down transports and servers")

	slog.Info("Shutting down http server", "addr", addr)
	err := server.Shutdown(context.Background())
	if err != nil {
		slog.Error("There wan an error when shutting down the Web Server", "error", err.Error())
	}

	if c.MCPServer != nil {
		if err = c.MCPServer.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down MCP server", "error", err.Error())
		}
	}
	for _, t := range c.Transports {
		if err = t.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down transport server", "error", err.Error())
		}
	}
	return nil
}

func (c *Coordinator) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", c.HandleHome)
	r.Get("/devices/{id}", c.HandleDeviceDetail)
	// r.Post("/devices/{id}/execute", c.HandleExecuteAction)
	return r
}

func (c *Coordinator) RegisterTransport(t Transport) {
	t.OnMessage(c.Handle)
	t.OnConnect(c.RegisterDevice)
	t.OnDisconnect(c.RemoveDevice)
	c.Transports = append(c.Transports, t)
}

func (c *Coordinator) RegisterDevice(client Client) error {
	c.Registery.Store(client)

	slog.Info("Registered client", "id", client.Meta().Id)
	return nil
}

func (c *Coordinator) RemoveDevice(client Client) {
	c.Registery.Delete(client.Meta().Id)

	c.topicSourcesMu.Lock()
	for _, capability := range client.Meta().Capabilities {
		if _, ok := c.topicSources[capability.Name]; !ok {
			slog.Error("Capability name/topic does not exist in topic sources", "topic", capability.Name)
			continue
		}
		delete(c.topicSources, capability.Name)
	}
	c.topicSourcesMu.Unlock()
}

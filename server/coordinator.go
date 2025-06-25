package server

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

type Coordinator struct {
	Registery  *DeviceRegistry
	Broker     *Broker
	MCPServer  *MCPServer
	Transports []Transport
}

func NewCoordinator(registery *DeviceRegistry, broker *Broker, mcpServer *MCPServer) *Coordinator {
	if mcpServer != nil {
		list_clients := mcp.NewTool("list_devices", mcp.WithDescription("Get a list of the devices connected to this server"))
		mcpServer.AddTool(list_clients, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			type ClientElement struct {
				Id   string `json:"id"`
				Name string `json:"name"`
			}
			clients := registery.List()
			res := make([]ClientElement, 0, len(clients))
			for _, client := range clients {
				res = append(res, ClientElement{Id: client.Meta().Id, Name: client.Meta().Name})
			}

			jsonBytes, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return nil, err
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: string(jsonBytes),
					},
				}}, nil
		})
	}

	return &Coordinator{Registery: registery, Broker: broker, MCPServer: mcpServer}
}

func (c *Coordinator) Start(ctx context.Context) error {
	// TODO: Add context to check if go routines exit for some reason
	if c.MCPServer != nil {
		go c.MCPServer.Start()
	}
	for _, t := range c.Transports {
		go t.Start()
	}

	<-ctx.Done()
	slog.Info("Shutting down transports and server")

	if c.MCPServer != nil {
		if err := c.MCPServer.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down MCP server", "error", err.Error())
		}
	}
	for _, t := range c.Transports {
		if err := t.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down transport server", "error", err.Error())
		}
	}
	return nil
}

func (c *Coordinator) RegisterTransport(t Transport) {
	t.OnMessage(c.Handle)
	t.OnConnect((c.RegisterDevice))
	t.OnDisconnect(func(client Client) { c.Registery.Delete(client.Meta().Id) })
	c.Transports = append(c.Transports, t)
}

func (c *Coordinator) RegisterDevice(client Client) error {
	c.Registery.Store(client)

	slog.Info("Registered client", "id", client.Meta().Id)
	return nil
}

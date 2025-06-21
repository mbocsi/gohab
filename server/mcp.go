package server

import (
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

type MCPServer struct {
	Server *server.MCPServer
}

func NewMCPServer() *MCPServer {
	return &MCPServer{Server: server.NewMCPServer("MCP Server", "1.0.0")}
}

func (s *MCPServer) Start() error {
	slog.Info("Started stdio MCP server")
	defer func() {
		slog.Info("Shut down stdio MCP server")
	}()
	return server.ServeStdio(s.Server)
}

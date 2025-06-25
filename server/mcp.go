package server

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MCPServer struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	addr       string
}

func NewMCPServer(name string, addr string) *MCPServer {
	mcpServer := server.NewMCPServer(
		name,
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	httpServer := server.NewStreamableHTTPServer(mcpServer)
	return &MCPServer{mcpServer: mcpServer, httpServer: httpServer, addr: addr}
}

func (s *MCPServer) Start() error {
	slog.Info("Started HTTP MCP server")
	defer func() {
		slog.Info("Shut down HTTP MCP server")
	}()

	if err := s.httpServer.Start(s.addr); err != nil {
		slog.Error("Error when starting MCP http server", "error", err)
		return err
	}
	return nil
}

func (s *MCPServer) Shutdown() error {
	ctx := context.Background()
	return s.httpServer.Shutdown(ctx)
}

func (s *MCPServer) AddTool(tool mcp.Tool, fn func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	s.mcpServer.AddTool(tool, fn)
}

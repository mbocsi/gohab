package mcp

import (
	"context"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MCPTransport interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type HTTPMCPTransport struct {
	server *server.StreamableHTTPServer
	addr   string
}

func (h *HTTPMCPTransport) Start(ctx context.Context) error {
	return h.server.Start(h.addr)
}

func (h *HTTPMCPTransport) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

type StdioMCPTransport struct {
	server *server.StdioServer
}

func (s *StdioMCPTransport) Start(ctx context.Context) error {
	return s.server.Listen(ctx, os.Stdin, os.Stdout)
}

func (s *StdioMCPTransport) Shutdown(ctx context.Context) error {
	return nil
}

type MCPServer struct {
	mcpServer *server.MCPServer
	transport MCPTransport
}

// Factory functions that handle dependency coordination
func NewMCPServerWithHTTP(name, addr string) *MCPServer {
	baseMCPServer := server.NewMCPServer(
		name,
		"1.0.0",
		server.WithToolCapabilities(false),
	)
	
	httpServer := server.NewStreamableHTTPServer(baseMCPServer)
	transport := &HTTPMCPTransport{
		server: httpServer,
		addr:   addr,
	}
	
	return &MCPServer{
		mcpServer: baseMCPServer,
		transport: transport,
	}
}

func NewMCPServerWithStdio(name string) *MCPServer {
	baseMCPServer := server.NewMCPServer(
		name,
		"1.0.0",
		server.WithToolCapabilities(false),
	)
	
	stdioServer := server.NewStdioServer(baseMCPServer)
	transport := &StdioMCPTransport{
		server: stdioServer,
	}
	
	return &MCPServer{
		mcpServer: baseMCPServer,
		transport: transport,
	}
}

func (s *MCPServer) Start() error {
	slog.Info("Starting MCP server")
	defer func() {
		slog.Info("Shut down MCP server")
	}()

	ctx := context.Background()
	if err := s.transport.Start(ctx); err != nil {
		slog.Error("Error when starting MCP server", "error", err)
		return err
	}
	return nil
}

func (s *MCPServer) Shutdown() error {
	ctx := context.Background()
	return s.transport.Shutdown(ctx)
}

func (s *MCPServer) AddTool(tool mcp.Tool, fn func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	s.mcpServer.AddTool(tool, fn)
}

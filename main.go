package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mbocsi/gohab/broker"
	"github.com/mbocsi/gohab/mcp"
	"github.com/mbocsi/gohab/transport"
)

type App struct {
	Clients    map[string]transport.Client
	Broker     *broker.Broker
	MCPServer  mcp.Server
	Transports []transport.Transport
}

func NewApp(broker *broker.Broker, mcpServer mcp.Server) *App {
	return &App{Clients: make(map[string]transport.Client), Broker: broker, MCPServer: mcpServer}
}

func (a *App) RegisterTransport(transport transport.Transport) {
	a.Transports = append(a.Transports, transport)
}

func (a *App) Start(ctx context.Context) {
	go a.MCPServer.Run()
	for _, t := range a.Transports {
		go t.Start()
	}

	<-ctx.Done()
	slog.Info("Shutting down transports and server")

	for _, t := range a.Transports {
		t.Shutdown()
	}
}

func main() {
	logger := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logger))

	broker := broker.NewBroker()

	tcpServer := transport.NewTCPTransport("0.0.0.0:8080")
	tcpServer.OnMessage(broker.Publish)

	mcpServer := mcp.NewMCPServer()

	app := NewApp(broker, mcpServer)
	app.RegisterTransport(tcpServer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.Start(ctx)
}

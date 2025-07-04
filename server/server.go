package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

type Server interface {
	Start() error
	Shutdown() error
}

type GohabServerOptions struct {
	MCPServer *MCPServer      // Optional MCPServer to run alongside
	Broker    *Broker         // Optional (defaults to new Broker if nil)
	Registry  *DeviceRegistry // Optional (defaults to new Registry if nil)
	Context   context.Context // Optional (defaults to context.Background())
	Addr      string
}

type GohabServer struct {
	options     GohabServerOptions
	coordinator *Coordinator
}

func NewGohabServer(opts GohabServerOptions) *GohabServer {
	if opts.Broker == nil {
		opts.Broker = NewBroker()
	}
	if opts.Registry == nil {
		opts.Registry = NewDeviceRegistry()
	}
	// TODO: Currently not used
	if opts.Context == nil {
		opts.Context = context.Background()
	}

	coordinator := NewCoordinator(opts.Registry, opts.Broker, opts.MCPServer)

	return &GohabServer{
		options:     opts,
		coordinator: coordinator,
	}
}

func (s *GohabServer) RegisterTransport(t Transport) {
	s.coordinator.RegisterTransport(t)
}

func setupLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

func (s *GohabServer) Start(addr string) error {
	setupLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s.coordinator.Start(ctx, addr)
	return nil
}

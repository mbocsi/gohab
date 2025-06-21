package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mbocsi/gohab/app"
	"github.com/mbocsi/gohab/broker"
	"github.com/mbocsi/gohab/mcp"
	"github.com/mbocsi/gohab/transport"
)

func setupLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

func main() {
	setupLogger()

	broker := broker.NewBroker()

	tcpServer := transport.NewTCPTransport("0.0.0.0:8080")

	mcpServer := mcp.NewMCPServer()

	devRegistery := app.NewDeviceRegistery()

	app := app.NewApp(devRegistery, broker, mcpServer)
	app.RegisterTransport(tcpServer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.Start(ctx)
}

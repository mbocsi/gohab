package main

import (
	"log/slog"

	"github.com/mbocsi/gohab/server"
)

func main() {
	tcpServer := server.NewTCPTransport("0.0.0.0:8080")
	mcpServer := server.NewMCPServer()
	gohabServer := server.NewGohabServer(server.GohabServerOptions{MCPServer: mcpServer})
	gohabServer.RegisterTransport(tcpServer)

	if err := gohabServer.Start(); err != nil {
		slog.Error("Error starting gohab server", "error", err.Error())
	}
}

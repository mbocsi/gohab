package main

import (
	"log/slog"

	"github.com/mbocsi/gohab/server"
)

func main() {
	tcpServer := server.NewTCPTransport("0.0.0.0:8888")
	gohabServer := server.NewGohabServer(server.GohabServerOptions{})
	gohabServer.RegisterTransport(tcpServer)

	if err := gohabServer.Start(":8080"); err != nil {
		slog.Error("Error starting gohab server", "error", err.Error())
	}
}

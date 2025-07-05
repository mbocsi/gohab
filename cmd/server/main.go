package main

import (
	"log/slog"

	"github.com/mbocsi/gohab/server"
)

func main() {
	tcpServer := server.NewTCPTransport("0.0.0.0:8888")
	tcpServer.SetName("Main TCP server")
	tcp2Server := server.NewTCPTransport("0.0.0.0:8889")
	tcp2Server.SetName("Backup TCP server")
	gohabServer := server.NewGohabServer(server.GohabServerOptions{})
	gohabServer.RegisterTransport(tcpServer)
	gohabServer.RegisterTransport(tcp2Server)

	if err := gohabServer.Start(":8080"); err != nil {
		slog.Error("Error starting gohab server", "error", err.Error())
	}
}

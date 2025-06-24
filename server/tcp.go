package server

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"

	"github.com/mbocsi/gohab/proto"
)

type TCPTransport struct {
	Addr         string
	listener     net.Listener
	onMessage    func(proto.Message)
	onConnect    func(Client) error
	onDisconnect func(Client)
}

func NewTCPTransport(addr string) *TCPTransport {
	return &TCPTransport{Addr: addr}
}

func (t *TCPTransport) Start() error {
	slog.Info("Starting tcp server", "addr", t.Addr)

	l, err := net.Listen("tcp", t.Addr)
	if err != nil {
		return nil
	}
	t.listener = l
	defer l.Close()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return err // exits goroutine when listener is closed
		}
		go t.handleConnection(conn)
	}
}

func (t *TCPTransport) handleConnection(c net.Conn) {
	ip := c.RemoteAddr().String()
	slog.Info("Device connected", "addr", ip)

	id := generateClientId("tcp")
	client := NewTCPClient(c, DeviceMetadata{Id: id})

	defer func() {
		if t.onDisconnect != nil {
			t.onDisconnect(client)
		} else {
			panic("TCPTransport callback is not defined")
		}
		slog.Info("Device disconnected", "addr", ip, "id", client.Id)
		c.Close()
	}()

	reader := bufio.NewScanner(c)

	if t.onConnect == nil {
		panic("TCPTransport onConnect callback is not defined")
	}
	err := t.onConnect(client)
	if err != nil {
		slog.Error("Failed to register device", "addr", ip, "error", err.Error())
		return
	}

	for reader.Scan() {
		line := reader.Bytes()
		var msg proto.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("Invalid JSON message received", "error", err, "data", string(line))
			continue
		}
		// Inject client ID into message
		msg.Sender = client.Id
		slog.Debug("Message received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "size", len(msg.Payload))
		if t.onMessage != nil {
			t.onMessage(msg)
		} else {
			panic("TCPTransport onMessage callback is not defined")
		}
	}
}

func (t *TCPTransport) Shutdown() error {
	slog.Info("Shutting down tcp server", "addr", t.Addr)
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

func (t *TCPTransport) OnMessage(fn func(proto.Message)) {
	t.onMessage = fn
}

func (t *TCPTransport) OnConnect(fn func(Client) error) {
	t.onConnect = fn
}

func (t *TCPTransport) OnDisconnect(fn func(Client)) {
	t.onDisconnect = fn
}

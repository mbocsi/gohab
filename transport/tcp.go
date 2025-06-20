package transport

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
)

type TCPTransport struct {
	Addr         string
	listener     net.Listener
	onMessage    func(Message)
	onConnect    func(Client)
	onDisconnect func(Client)
}

func NewTCPTransport(addr string) *TCPTransport {
	return &TCPTransport{Addr: addr}
}

func (t *TCPTransport) Start() error {
	slog.Debug("Starting tcp server", "addr", t.Addr)

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
	id := c.RemoteAddr().String()
	slog.Info("Device connected", "addr", id)

	client := NewTCPClient(id, c)
	if t.onConnect != nil {
		t.onConnect(client)
	}

	defer func() {
		if t.onDisconnect != nil {
			t.onDisconnect(client)
		}
		slog.Info("Device disconnected", "addr", id)
		c.Close()
	}()

	reader := bufio.NewScanner(c)

	for reader.Scan() {
		line := reader.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("Invalid JSON message", "addr", id, "error", err, "data", string(line))
			continue
		}
		if t.onMessage != nil {
			t.onMessage(msg)
		}
	}
}

func (t *TCPTransport) Shutdown() error {
	slog.Debug("Shutting down tcp server", "addr", t.Addr)
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

func (t *TCPTransport) OnMessage(fn func(Message)) {
	t.onMessage = fn
}

func (t *TCPTransport) OnConnect(fn func(Client)) {
	t.onConnect = fn
}

func (t *TCPTransport) OnDisconnect(fn func(Client)) {
	t.onDisconnect = fn
}

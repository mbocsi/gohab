package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/mbocsi/gohab/proto"
)

type TCPTransport struct {
	Addr         string
	listener     net.Listener
	onMessage    func(proto.Message)
	onConnect    func(Client) error
	onDisconnect func(Client)

	name        string
	description string
	clients     map[string]Client
	cmu         sync.RWMutex

	maxClients int
	connected  bool
}

func NewTCPTransport(addr string) *TCPTransport {
	return &TCPTransport{Addr: addr, maxClients: 16, clients: make(map[string]Client)}
}

func (t *TCPTransport) Start() error {
	slog.Info("Starting tcp server", "addr", t.Addr)

	if t.onConnect == nil || t.onDisconnect == nil || t.onMessage == nil {
		return fmt.Errorf("The OnConnect, OnDisconnect, or OnMessage function is not defined. This transport is likely being called outside of the server coordinator.")
	}

	l, err := net.Listen("tcp", t.Addr)
	if err != nil {
		return err
	}
	t.listener = l
	t.connected = true
	defer func() {
		l.Close()
		t.connected = false
	}()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return err // exits goroutine when listener is closed
		}

		t.cmu.RLock()
		clientCount := len(t.clients)
		t.cmu.RUnlock()

		if clientCount >= t.maxClients {
			slog.Warn("Max clients reached, rejecting connection", "remote_addr", conn.RemoteAddr())
			conn.Close() // Reject connection politely
			continue
		}

		go t.handleConnection(conn)
	}
}

func (t *TCPTransport) handleConnection(c net.Conn) {
	ip := c.RemoteAddr().String()
	slog.Info("Device connected", "addr", ip)

	client := NewTCPClient(c, t)

	defer func() {
		t.cmu.Lock()
		delete(t.clients, client.Id)
		t.cmu.Unlock()

		t.onDisconnect(client)

		c.Close()
		slog.Info("Device disconnected", "addr", ip, "id", client.Id)
	}()

	reader := bufio.NewScanner(c)

	err := t.onConnect(client)
	if err != nil {
		slog.Error("Failed to register device", "addr", ip, "error", err.Error())
		return
	}
	t.cmu.Lock()
	t.clients[client.Id] = client
	t.cmu.Unlock()

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
		t.onMessage(msg)
	}

	if err := reader.Err(); err != nil {
		slog.Warn("Connection error", "addr", ip, "error", err)
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

func (t *TCPTransport) Meta() TransportMetadata {
	t.cmu.RLock()
	clients := t.clients
	t.cmu.RUnlock()
	return TransportMetadata{
		ID:          "tcp-" + t.Addr,
		Name:        t.name,
		Description: t.description,
		Protocol:    "tcp",
		Address:     t.Addr,
		Clients:     clients,
		MaxClients:  t.maxClients,
		Connected:   t.connected,
	}
}

func (t *TCPTransport) SetName(name string) {
	t.name = name
}

func (t *TCPTransport) SetMaxClients(n int) {
	t.maxClients = n
}

func (t *TCPTransport) SetDescription(description string) {
	t.description = description
}

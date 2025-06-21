package transport

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"time"
)

type TCPTransport struct {
	Addr         string
	listener     net.Listener
	onMessage    func(Message)
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
	id := c.RemoteAddr().String()
	slog.Info("Device connected", "addr", id)

	client := NewTCPClient(c, DeviceMetadata{Id: id})

	defer func() {
		if t.onDisconnect != nil {
			t.onDisconnect(client)
		} else {
			panic("TCPTransport callback is not defined")
		}
		slog.Info("Device disconnected", "addr", id, "id", client.Id)
		c.Close()
	}()

	reader := bufio.NewScanner(c)

	// Identify device
	for reader.Scan() {
		line := reader.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("Invalid JSON message", "addr", id, "error", err.Error(), "data", string(line))
			continue
		}
		if msg.Type != "identify" {
			slog.Warn("Received a message other than identify", "addr", id, "data", string(line))
			continue
		}
		if t.onConnect == nil {
			panic("TCPTransport onConnect callback is not defined")
		}

		var idPayload IdentifyPayload
		if err := json.Unmarshal(msg.Payload, &idPayload); err != nil {
			slog.Warn("Invalid JSON identify payload", "addr", id, "error", err.Error(), "data", string(line))
			continue
		}

		client.DeviceMetadata = DeviceMetadata{Name: idPayload.ProposedName,
			LastSeen:     time.Now(),
			Firmware:     idPayload.Firmware,
			Capabilities: idPayload.Capabilities}

		err := t.onConnect(client)
		if err != nil {
			slog.Warn("Failed to register device identity", "addr", id, "error", err.Error(), "data", string(line))
		} else {
			// identity register success
			break
		}
	}

	slog.Info("Identified client", "addr", id, "id", client.Id)

	// Read messages from device
	for reader.Scan() {
		line := reader.Bytes()
		slog.Debug("Received data", "data", string(line))
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("Invalid JSON message", "addr", id, "error", err, "data", string(line))
			continue
		}
		// Inject client ID into message
		msg.Sender = client.Id
		if t.onMessage != nil {
			t.onMessage(msg)
		} else {
			panic("TCPTransport onMessage callback is not defined")
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

func (t *TCPTransport) OnConnect(fn func(Client) error) {
	t.onConnect = fn
}

func (t *TCPTransport) OnDisconnect(fn func(Client)) {
	t.onDisconnect = fn
}

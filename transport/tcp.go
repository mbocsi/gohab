package transport

import (
	"encoding/json"
	"log/slog"
	"net"
)

type TCPTransport struct {
	Addr      string
	listener  net.Listener
	onMessage func(Message)
}

func NewTCPTransport(addr string) *TCPTransport {
	return &TCPTransport{Addr: addr, listener: nil}
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
	slog.Info("Device connected: %s", "addr", c.RemoteAddr().String())

	buf := make([]byte, 4096)
	msg := Message{}
	defer c.Close()

	for {
		n, err := c.Read(buf)
		if err != nil {
			break
		}
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			slog.Warn("TCP message malformed", "error", err.Error())
			continue
		}
		t.onMessage(msg)
	}
	slog.Info("Device disconnected: %s", "addr", c.RemoteAddr().String())
}

func (t *TCPTransport) Shutdown() error {
	slog.Debug("Shutting down tcp server", "addr", t.Addr)
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

func (t *TCPTransport) OnMessage(handler func(Message)) {
	t.onMessage = handler
}

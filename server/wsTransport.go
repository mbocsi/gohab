package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mbocsi/gohab/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

type WSTransport struct {
	Addr         string
	server       *http.Server
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

func NewWSTransport(addr string) *WSTransport {
	return &WSTransport{
		Addr:       addr,
		maxClients: 16,
		clients:    make(map[string]Client),
	}
}

func (t *WSTransport) Start() error {
	slog.Info("Starting WebSocket server", "addr", t.Addr)

	if t.onConnect == nil || t.onDisconnect == nil || t.onMessage == nil {
		return fmt.Errorf("The OnConnect, OnDisconnect, or OnMessage function is not defined. This transport is likely being called outside of the server coordinator.")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleWebSocket)

	t.server = &http.Server{
		Addr:    t.Addr,
		Handler: mux,
	}

	t.connected = true
	err := t.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		t.connected = false
		return err
	}

	return nil
}

func (t *WSTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection", "error", err)
		return
	}

	t.cmu.RLock()
	clientCount := len(t.clients)
	t.cmu.RUnlock()

	if clientCount >= t.maxClients {
		slog.Warn("Max clients reached, rejecting connection", "remote_addr", r.RemoteAddr)
		conn.Close()
		return
	}

	go t.handleConnection(conn, r.RemoteAddr)
}

func (t *WSTransport) handleConnection(conn *websocket.Conn, remoteAddr string) {
	slog.Info("WebSocket device connected", "addr", remoteAddr)

	client := NewWSClient(conn, t)

	defer func() {
		t.cmu.Lock()
		delete(t.clients, client.Id)
		t.cmu.Unlock()

		t.onDisconnect(client)

		conn.Close()
		slog.Info("WebSocket device disconnected", "addr", remoteAddr, "id", client.Id)
	}()

	err := t.onConnect(client)
	if err != nil {
		slog.Error("Failed to register WebSocket device", "addr", remoteAddr, "error", err.Error())
		return
	}

	t.cmu.Lock()
	t.clients[client.Id] = client
	t.cmu.Unlock()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket connection error", "addr", remoteAddr, "error", err)
			}
			break
		}

		var msg proto.Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			slog.Warn("Invalid JSON message received", "error", err, "data", string(messageBytes))
			continue
		}

		// Inject client ID into message
		msg.Sender = client.Id
		slog.Debug("WebSocket message received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "size", len(msg.Payload))
		t.onMessage(msg)
	}
}

func (t *WSTransport) Shutdown() error {
	slog.Info("Shutting down WebSocket server", "addr", t.Addr)
	t.connected = false
	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

func (t *WSTransport) OnMessage(fn func(proto.Message)) {
	t.onMessage = fn
}

func (t *WSTransport) OnConnect(fn func(Client) error) {
	t.onConnect = fn
}

func (t *WSTransport) OnDisconnect(fn func(Client)) {
	t.onDisconnect = fn
}

func (t *WSTransport) Meta() TransportMetadata {
	t.cmu.RLock()
	clients := t.clients
	t.cmu.RUnlock()
	return TransportMetadata{
		ID:          "ws-" + t.Addr,
		Name:        t.name,
		Description: t.description,
		Protocol:    "websocket",
		Address:     t.Addr,
		Clients:     clients,
		MaxClients:  t.maxClients,
		Connected:   t.connected,
	}
}

func (t *WSTransport) SetName(name string) {
	t.name = name
}

func (t *WSTransport) SetMaxClients(n int) {
	t.maxClients = n
}

func (t *WSTransport) SetDescription(description string) {
	t.description = description
}
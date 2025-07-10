package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/mbocsi/gohab/proto"
)

type WebSocketTransport struct {
	conn *websocket.Conn
}

func NewWebSocketTransport() *WebSocketTransport {
	return &WebSocketTransport{}
}

func (t *WebSocketTransport) Connect(addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	// If no scheme is provided, assume ws://
	if u.Scheme == "" {
		u.Scheme = "ws"
	}

	// Convert tcp addresses to WebSocket URLs
	if u.Scheme == "tcp" {
		u.Scheme = "ws"
		u.Path = "/"
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket server: %w", err)
	}

	t.conn = conn
	return nil
}

func (t *WebSocketTransport) Send(msg proto.Message) error {
	if t.conn == nil {
		return fmt.Errorf("transport is not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = t.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return fmt.Errorf("failed to send WebSocket message: %w", err)
	}

	slog.Debug("Sent WebSocket Message", "type", msg.Type, "topic", msg.Topic, "size", len(msg.Payload))
	return nil
}

func (t *WebSocketTransport) Read() (proto.Message, error) {
	if t.conn == nil {
		return proto.Message{}, fmt.Errorf("transport is not connected")
	}

	_, messageBytes, err := t.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			return proto.Message{}, fmt.Errorf("WebSocket connection error: %w", err)
		}
		return proto.Message{}, fmt.Errorf("connection closed: %w", err)
	}

	var msg proto.Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return proto.Message{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return msg, nil
}

func (t *WebSocketTransport) Close() error {
	if t.conn == nil {
		return nil
	}

	// Send close message
	err := t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		// Log error but don't return it - we still want to close the connection
		slog.Warn("Failed to send close message", "error", err)
	}

	return t.conn.Close()
}
package server

import (
	"encoding/json"
	"log/slog"

	"github.com/gorilla/websocket"
	"github.com/mbocsi/gohab/proto"
)

type WSClient struct {
	DeviceMetadata
	conn *websocket.Conn
}

func NewWSClient(conn *websocket.Conn, t Transport) *WSClient {
	return &WSClient{
		conn:           conn,
		DeviceMetadata: DeviceMetadata{Id: generateClientId("ws"), Subs: make(map[string]struct{}), Transport: t},
	}
}

func (c *WSClient) Send(msg proto.Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = c.conn.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		return err
	}

	slog.Debug("Sent WebSocket Message", "to", c.Meta().Id, "type", msg.Type, "topic", msg.Topic, "size", len(msg.Payload))
	return nil
}

func (c *WSClient) Meta() *DeviceMetadata {
	return &c.DeviceMetadata
}
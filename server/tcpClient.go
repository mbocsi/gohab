package server

import (
	"encoding/json"
	"log/slog"
	"net"

	"github.com/mbocsi/gohab/proto"
)

type TCPClient struct {
	DeviceMetadata
	conn      net.Conn
	mcpServer *MCPServer // Currently not used
}

func NewTCPClient(conn net.Conn) *TCPClient {
	return &TCPClient{conn: conn, DeviceMetadata: DeviceMetadata{Id: generateClientId("tcp"), Subs: make(map[string]struct{})}}
}

func (c *TCPClient) Send(msg proto.Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	jsonData = append(jsonData, '\n') // New line to indicate end of message
	_, err = c.conn.Write(jsonData)
	slog.Debug("Sent Message", "to", c.Meta().Id, "type", msg.Type, "topic", msg.Topic, "size", len(msg.Payload))
	return err
}

func (c *TCPClient) Meta() *DeviceMetadata {
	return &c.DeviceMetadata
}

func (c *TCPClient) MCPServer() *MCPServer {
	return c.mcpServer
}

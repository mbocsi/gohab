package transport

import (
	"encoding/json"
	"net"
)

type TCPClient struct {
	DeviceMetadata
	conn net.Conn
}

func NewTCPClient(conn net.Conn, meta DeviceMetadata) *TCPClient {
	return &TCPClient{conn: conn, DeviceMetadata: meta}
}

func (c *TCPClient) Send(msg Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(jsonData)
	return err
}

func (c *TCPClient) Meta() *DeviceMetadata {
	return &c.DeviceMetadata
}

package transport

import (
	"encoding/json"
	"net"
)

type TCPClient struct {
	id   string
	conn net.Conn
}

func NewTCPClient(id string, conn net.Conn) *TCPClient {
	return &TCPClient{id, conn}
}

func (c *TCPClient) Send(msg Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(jsonData)
	return err
}

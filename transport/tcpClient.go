package transport

import (
	"encoding/json"
	"net"
)

type TCPClient struct {
	Id   string
	conn net.Conn
}

func NewTCPClient(id string, conn net.Conn) *TCPClient {
	return &TCPClient{Id: id, conn: conn}
}

func (c *TCPClient) Send(msg Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(jsonData)
	return err
}

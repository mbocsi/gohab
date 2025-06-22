package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"

	"github.com/mbocsi/gohab/proto"
)

type TCPTransport struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func NewTCPTransport() *TCPTransport {
	return &TCPTransport{}
}

func (t *TCPTransport) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	t.conn = conn
	t.scanner = bufio.NewScanner(conn)
	return nil
}

func (t *TCPTransport) Send(msg proto.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = t.conn.Write(data)
	return err
}

func (t *TCPTransport) Read() (proto.Message, error) {
	for t.scanner.Scan() {
		var msg proto.Message
		if err := json.Unmarshal(t.scanner.Bytes(), &msg); err != nil {
			return proto.Message{}, fmt.Errorf("invalid JSON: %w", err)
		}
		return msg, nil
	}

	if err := t.scanner.Err(); err != nil {
		return proto.Message{}, err
	}

	return proto.Message{}, fmt.Errorf("connection closed")
}

func (t *TCPTransport) Close() error {
	return t.conn.Close()
}

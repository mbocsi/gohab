package client

import "github.com/mbocsi/gohab/proto"

type Transport interface {
	Connect(addr string) error
	Send(msg proto.Message) error
	Read() (proto.Message, error) // for one-at-a-time processing
	Close() error
}

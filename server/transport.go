package server

import (
	"time"

	"github.com/mbocsi/gohab/proto"
)

type Transport interface {
	Start() error
	OnMessage(func(proto.Message))
	OnConnect(func(Client) error)
	OnDisconnect(func(Client))
	Shutdown() error
}

type DeviceMetadata struct {
	Id           string
	Name         string
	LastSeen     time.Time
	Firmware     string
	Capabilities []proto.Capability
}

type Client interface {
	Send(proto.Message) error
	Meta() *DeviceMetadata
}

package app

import (
	"sync"

	"github.com/mbocsi/gohab/transport"
)

type DeviceRegistry struct {
	mu    sync.RWMutex
	store map[string]transport.Client
}

func NewDeviceRegistery() *DeviceRegistry {
	return &DeviceRegistry{store: make(map[string]transport.Client)}
}

func (r *DeviceRegistry) Store(client transport.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[client.Meta().Id] = client
}

func (r *DeviceRegistry) Get(id string) (transport.Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok := r.store[id]
	return val, ok
}

func (r *DeviceRegistry) Delete(id string) transport.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store[id]
}

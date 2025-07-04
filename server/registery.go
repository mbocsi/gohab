package server

import (
	"sync"
)

type DeviceRegistry struct {
	mu    sync.RWMutex
	store map[string]Client
}

func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{store: make(map[string]Client)}
}

func (r *DeviceRegistry) Store(client Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[client.Meta().Id] = client
}

func (r *DeviceRegistry) Get(id string) (Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok := r.store[id]
	return val, ok
}

func (r *DeviceRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
}

func (r *DeviceRegistry) List() []Client {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients := make([]Client, 0, len(r.store))
	for _, client := range r.store {
		clients = append(clients, client)
	}

	return clients
}

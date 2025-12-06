package server

import (
	"sync"
	"testing"
	"time"

	"github.com/mbocsi/gohab/proto"
)

// MockClientForRegistry implements Client for registry testing
type MockClientForRegistry struct {
	id       string
	metadata DeviceMetadata
}

func NewMockClientForRegistry(id string) *MockClientForRegistry {
	return &MockClientForRegistry{
		id: id,
		metadata: DeviceMetadata{
			Id:       id,
			Name:     "Test Device " + id,
			Firmware: "1.0.0",
			LastSeen: time.Now(),
			Features: make(map[string]proto.Feature),
			Subs:     make(map[string]struct{}),
		},
	}
}

func (mc *MockClientForRegistry) Send(msg proto.Message) error {
	return nil
}

func (mc *MockClientForRegistry) Meta() *DeviceMetadata {
	return &mc.metadata
}

func TestNewDeviceRegistry(t *testing.T) {
	registry := NewDeviceRegistry()
	
	if registry == nil {
		t.Fatal("Expected registry to be created")
	}
	
	if registry.store == nil {
		t.Error("Expected store map to be initialized")
	}
}

func TestDeviceRegistry_Store(t *testing.T) {
	registry := NewDeviceRegistry()
	client := NewMockClientForRegistry("test-device")
	
	registry.Store(client)
	
	stored, exists := registry.Get("test-device")
	if !exists {
		t.Error("Expected client to be stored")
	}
	
	if stored.Meta().Id != "test-device" {
		t.Errorf("Expected stored client ID 'test-device', got %s", stored.Meta().Id)
	}
}

func TestDeviceRegistry_Store_Update(t *testing.T) {
	registry := NewDeviceRegistry()
	client1 := NewMockClientForRegistry("device-1")
	client1.metadata.Name = "Original Name"
	
	registry.Store(client1)
	
	// Store updated client with same ID
	client2 := NewMockClientForRegistry("device-1")
	client2.metadata.Name = "Updated Name"
	
	registry.Store(client2)
	
	stored, exists := registry.Get("device-1")
	if !exists {
		t.Error("Expected client to exist after update")
	}
	
	if stored.Meta().Name != "Updated Name" {
		t.Errorf("Expected updated name 'Updated Name', got %s", stored.Meta().Name)
	}
	
	// Should have only one entry
	clients := registry.List()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client after update, got %d", len(clients))
	}
}

func TestDeviceRegistry_Get_NotFound(t *testing.T) {
	registry := NewDeviceRegistry()
	
	client, exists := registry.Get("nonexistent")
	if exists {
		t.Error("Expected client not to exist")
	}
	
	if client != nil {
		t.Error("Expected nil client for nonexistent ID")
	}
}

func TestDeviceRegistry_Delete(t *testing.T) {
	registry := NewDeviceRegistry()
	client := NewMockClientForRegistry("test-device")
	
	registry.Store(client)
	
	// Verify it exists
	_, exists := registry.Get("test-device")
	if !exists {
		t.Error("Expected client to exist before deletion")
	}
	
	registry.Delete("test-device")
	
	// Verify it's gone
	_, exists = registry.Get("test-device")
	if exists {
		t.Error("Expected client to be deleted")
	}
}

func TestDeviceRegistry_Delete_NotFound(t *testing.T) {
	registry := NewDeviceRegistry()
	
	// Should not panic when deleting nonexistent client
	registry.Delete("nonexistent")
	
	// Registry should still be usable
	client := NewMockClientForRegistry("test-device")
	registry.Store(client)
	
	_, exists := registry.Get("test-device")
	if !exists {
		t.Error("Registry should still work after deleting nonexistent client")
	}
}

func TestDeviceRegistry_List_Empty(t *testing.T) {
	registry := NewDeviceRegistry()
	
	clients := registry.List()
	if clients == nil {
		t.Error("Expected empty slice, got nil")
	}
	
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(clients))
	}
}

func TestDeviceRegistry_List_Multiple(t *testing.T) {
	registry := NewDeviceRegistry()
	
	// Store multiple clients
	client1 := NewMockClientForRegistry("device-1")
	client2 := NewMockClientForRegistry("device-2")
	client3 := NewMockClientForRegistry("device-3")
	
	registry.Store(client1)
	registry.Store(client2)
	registry.Store(client3)
	
	clients := registry.List()
	if len(clients) != 3 {
		t.Errorf("Expected 3 clients, got %d", len(clients))
	}
	
	// Verify all clients are present (order doesn't matter)
	ids := make(map[string]bool)
	for _, client := range clients {
		ids[client.Meta().Id] = true
	}
	
	expectedIDs := []string{"device-1", "device-2", "device-3"}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("Expected client %s to be in list", id)
		}
	}
}

func TestDeviceRegistry_List_ReturnsSnapshot(t *testing.T) {
	registry := NewDeviceRegistry()
	client := NewMockClientForRegistry("test-device")
	
	registry.Store(client)
	
	list1 := registry.List()
	list2 := registry.List()
	
	// Should return different slices
	if &list1[0] == &list2[0] {
		t.Error("List should return new slices, not references to internal state")
	}
	
	// Both should have same content
	if len(list1) != len(list2) {
		t.Error("Both lists should have same length")
	}
	
	if list1[0].Meta().Id != list2[0].Meta().Id {
		t.Error("Both lists should have same content")
	}
}

func TestDeviceRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewDeviceRegistry()
	numGoroutines := 10
	numOperations := 100
	
	var wg sync.WaitGroup
	
	// Concurrent stores
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				client := NewMockClientForRegistry(string(rune('A'+id)))
				registry.Store(client)
			}
		}(i)
	}
	
	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				registry.Get(string(rune('A' + id)))
			}
		}(i)
	}
	
	// Concurrent lists
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				registry.List()
			}
		}()
	}
	
	// Concurrent deletes
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				registry.Delete(string(rune('A' + id)))
			}
		}(i)
	}
	
	wg.Wait()
	
	// Test should complete without race conditions or panics
	clients := registry.List()
	if len(clients) > numGoroutines {
		t.Errorf("Expected at most %d clients, got %d", numGoroutines, len(clients))
	}
}

func TestDeviceRegistry_StoreAndDeleteCycle(t *testing.T) {
	registry := NewDeviceRegistry()
	
	// Store, delete, store again
	client1 := NewMockClientForRegistry("cycling-device")
	client1.metadata.Name = "First Instance"
	
	registry.Store(client1)
	
	stored, exists := registry.Get("cycling-device")
	if !exists || stored.Meta().Name != "First Instance" {
		t.Error("First store failed")
	}
	
	registry.Delete("cycling-device")
	
	_, exists = registry.Get("cycling-device")
	if exists {
		t.Error("Delete failed")
	}
	
	client2 := NewMockClientForRegistry("cycling-device")
	client2.metadata.Name = "Second Instance"
	
	registry.Store(client2)
	
	stored, exists = registry.Get("cycling-device")
	if !exists || stored.Meta().Name != "Second Instance" {
		t.Error("Second store failed")
	}
}

func TestDeviceRegistry_MetadataUpdates(t *testing.T) {
	registry := NewDeviceRegistry()
	client := NewMockClientForRegistry("updating-device")
	
	registry.Store(client)
	
	// Update metadata through the stored reference
	stored, _ := registry.Get("updating-device")
	stored.Meta().Mu.Lock()
	stored.Meta().Name = "Updated Name"
	stored.Meta().LastSeen = time.Now().Add(time.Hour)
	stored.Meta().Mu.Unlock()
	
	// Verify updates are reflected
	retrieved, _ := registry.Get("updating-device")
	retrieved.Meta().Mu.RLock()
	name := retrieved.Meta().Name
	retrieved.Meta().Mu.RUnlock()
	
	if name != "Updated Name" {
		t.Errorf("Expected updated name 'Updated Name', got %s", name)
	}
}
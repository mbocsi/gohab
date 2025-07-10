package server

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mbocsi/gohab/proto"
)

func TestNewWSTransport(t *testing.T) {
	addr := "localhost:0"
	transport := NewWSTransport(addr)
	
	if transport.Addr != addr {
		t.Errorf("Expected addr %s, got %s", addr, transport.Addr)
	}
	
	if transport.maxClients != 16 {
		t.Errorf("Expected maxClients 16, got %d", transport.maxClients)
	}
	
	if transport.clients == nil {
		t.Error("Expected clients map to be initialized")
	}
}

func TestWSTransport_SetMethods(t *testing.T) {
	transport := NewWSTransport("localhost:0")
	
	transport.SetName("test-ws-transport")
	transport.SetMaxClients(10)
	transport.SetDescription("Test WebSocket transport")
	
	meta := transport.Meta()
	
	if meta.Name != "test-ws-transport" {
		t.Errorf("Expected name 'test-ws-transport', got %s", meta.Name)
	}
	
	if meta.MaxClients != 10 {
		t.Errorf("Expected maxClients 10, got %d", meta.MaxClients)
	}
	
	if meta.Description != "Test WebSocket transport" {
		t.Errorf("Expected description 'Test WebSocket transport', got %s", meta.Description)
	}
}

func TestWSTransport_StartWithoutCallbacks(t *testing.T) {
	transport := NewWSTransport("localhost:0")
	
	// Start in a goroutine since Start() will block
	done := make(chan error, 1)
	go func() {
		done <- transport.Start()
	}()
	
	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Should still be running, so shutdown
	transport.Shutdown()
	
	// Wait for Start() to return
	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected error when starting without callbacks")
		}
	case <-time.After(1 * time.Second):
		t.Error("Start() did not return after shutdown")
	}
}

func TestWSTransport_Meta(t *testing.T) {
	transport := NewWSTransport("localhost:8080")
	transport.SetName("test-ws-transport")
	transport.SetDescription("Test WebSocket transport")
	transport.SetMaxClients(5)
	
	meta := transport.Meta()
	
	if meta.Protocol != "websocket" {
		t.Errorf("Expected protocol 'websocket', got %s", meta.Protocol)
	}
	
	if meta.Address != "localhost:8080" {
		t.Errorf("Expected address 'localhost:8080', got %s", meta.Address)
	}
	
	if meta.Connected != false {
		t.Errorf("Expected connected false, got %t", meta.Connected)
	}
	
	expectedID := "ws-localhost:8080"
	if meta.ID != expectedID {
		t.Errorf("Expected ID '%s', got %s", expectedID, meta.ID)
	}
}

func TestWSTransport_ClientConnection(t *testing.T) {
	// Use a specific port for testing
	transport := NewWSTransport("localhost:18889")
	
	var connectedClient Client
	var disconnectedClient Client
	var receivedMessage proto.Message
	
	transport.OnMessage(func(msg proto.Message) {
		receivedMessage = msg
	})
	
	transport.OnConnect(func(client Client) error {
		connectedClient = client
		return nil
	})
	
	transport.OnDisconnect(func(client Client) {
		disconnectedClient = client
	})
	
	// Start server in goroutine
	go func() {
		transport.Start()
	}()
	
	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	
	// Connect a test WebSocket client
	u := url.URL{Scheme: "ws", Host: "localhost:18889", Path: "/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	
	// Wait for connection to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Send a test message
	payloadData := map[string]interface{}{"data": "test"}
	payloadBytes, _ := json.Marshal(payloadData)
	testMsg := proto.Message{
		Type:    "test",
		Topic:   "test/topic",
		Payload: json.RawMessage(payloadBytes),
	}
	
	msgBytes, _ := json.Marshal(testMsg)
	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// Wait for message to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Close connection
	conn.Close()
	
	// Wait for disconnect to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Verify callbacks were called
	if connectedClient == nil {
		t.Error("OnConnect callback was not called")
	}
	
	if disconnectedClient == nil {
		t.Error("OnDisconnect callback was not called")
	}
	
	if receivedMessage.Type != "test" {
		t.Errorf("Expected message type 'test', got %s", receivedMessage.Type)
	}
	
	if receivedMessage.Topic != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got %s", receivedMessage.Topic)
	}
	
	// Verify sender was injected
	if receivedMessage.Sender == "" {
		t.Error("Expected sender to be set")
	}
	
	transport.Shutdown()
}
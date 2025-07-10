package server

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mbocsi/gohab/proto"
)

func TestNewTCPTransport(t *testing.T) {
	addr := "localhost:0"
	transport := NewTCPTransport(addr)
	
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

func TestTCPTransport_SetMethods(t *testing.T) {
	transport := NewTCPTransport("localhost:0")
	
	transport.SetName("test-transport")
	transport.SetMaxClients(10)
	transport.SetDescription("Test transport")
	
	meta := transport.Meta()
	
	if meta.Name != "test-transport" {
		t.Errorf("Expected name 'test-transport', got %s", meta.Name)
	}
	
	if meta.MaxClients != 10 {
		t.Errorf("Expected maxClients 10, got %d", meta.MaxClients)
	}
	
	if meta.Description != "Test transport" {
		t.Errorf("Expected description 'Test transport', got %s", meta.Description)
	}
}

func TestTCPTransport_StartWithoutCallbacks(t *testing.T) {
	transport := NewTCPTransport("localhost:0")
	
	err := transport.Start()
	if err == nil {
		t.Error("Expected error when starting without callbacks")
	}
}

func TestTCPTransport_StartAndShutdown(t *testing.T) {
	transport := NewTCPTransport("localhost:0")
	
	// Set up callbacks
	transport.OnMessage(func(msg proto.Message) {})
	transport.OnConnect(func(client Client) error { return nil })
	transport.OnDisconnect(func(client Client) {})
	
	go func() {
		err := transport.Start()
		// Only accept errors from listener.Accept() being closed (line 55 in tcpTransport.go)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			t.Errorf("Unexpected error during start: %v", err)
		}
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	err := transport.Shutdown()
	if err != nil {
		t.Errorf("Error during shutdown: %v", err)
	}
}

func TestTCPTransport_ClientConnection(t *testing.T) {
	transport := NewTCPTransport("localhost:0")
	
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
	
	go func() {
		transport.Start()
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Connect a test client
	conn, err := net.Dial("tcp", transport.listener.Addr().String())
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
	conn.Write(append(msgBytes, '\n'))
	
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

func TestTCPTransport_MaxClients(t *testing.T) {
	transport := NewTCPTransport("localhost:0")
	transport.SetMaxClients(1)
	
	transport.OnMessage(func(msg proto.Message) {})
	transport.OnConnect(func(client Client) error { return nil })
	transport.OnDisconnect(func(client Client) {})
	
	go func() {
		transport.Start()
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Connect first client
	conn1, err := net.Dial("tcp", transport.listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer conn1.Close()
	
	// Wait for connection to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Try to connect second client - should be rejected
	conn2, err := net.Dial("tcp", transport.listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect second client: %v", err)
	}
	
	// Try to read from second connection - should be closed immediately
	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	reader := bufio.NewReader(conn2)
	_, err = reader.ReadByte()
	
	// Connection should be closed, so we expect an error
	if err == nil {
		t.Error("Expected second connection to be closed due to max clients limit")
	}
	
	conn2.Close()
	transport.Shutdown()
}

func TestTCPTransport_Meta(t *testing.T) {
	transport := NewTCPTransport("localhost:8080")
	transport.SetName("test-transport")
	transport.SetDescription("Test TCP transport")
	transport.SetMaxClients(5)
	
	meta := transport.Meta()
	
	if meta.Protocol != "tcp" {
		t.Errorf("Expected protocol 'tcp', got %s", meta.Protocol)
	}
	
	if meta.Address != "localhost:8080" {
		t.Errorf("Expected address 'localhost:8080', got %s", meta.Address)
	}
	
	if meta.Connected != false {
		t.Errorf("Expected connected false, got %t", meta.Connected)
	}
	
	expectedID := "tcp-localhost:8080"
	if meta.ID != expectedID {
		t.Errorf("Expected ID '%s', got %s", expectedID, meta.ID)
	}
}
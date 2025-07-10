package server

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mbocsi/gohab/proto"
)

func TestNewTCPClient(t *testing.T) {
	// Create a mock transport
	mockTransport := &MockTransport{}

	// Create a pipe for testing
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewTCPClient(serverConn, mockTransport)

	// Verify client was created correctly
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.conn != serverConn {
		t.Error("Expected connection to be set")
	}

	if client.DeviceMetadata.Transport != mockTransport {
		t.Error("Expected transport to be set")
	}

	if !strings.HasPrefix(client.DeviceMetadata.Id, "tcp-") {
		t.Errorf("Expected ID to start with 'tcp-', got %s", client.DeviceMetadata.Id)
	}

	if client.DeviceMetadata.Subs == nil {
		t.Error("Expected subscriptions map to be initialized")
	}
}

func TestTCPClient_Send(t *testing.T) {
	// Create a mock transport
	mockTransport := &MockTransport{}

	// Create a pipe for testing
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewTCPClient(serverConn, mockTransport)

	// Create test message
	payloadData := map[string]interface{}{"test": "data"}
	payloadBytes, _ := json.Marshal(payloadData)
	testMsg := proto.Message{
		Type:      "test",
		Topic:     "test/topic",
		Payload:   json.RawMessage(payloadBytes),
		Timestamp: time.Now().Unix(),
	}

	// Send message in goroutine
	go func() {
		err := client.Send(testMsg)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}()

	// Read message from client side
	buffer := make([]byte, 1024)
	n, err := clientConn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from connection: %v", err)
	}

	// Verify message was sent correctly
	receivedData := buffer[:n]

	// Should end with newline
	if receivedData[len(receivedData)-1] != '\n' {
		t.Error("Expected message to end with newline")
	}

	// Parse JSON
	var receivedMsg proto.Message
	err = json.Unmarshal(receivedData[:len(receivedData)-1], &receivedMsg)
	if err != nil {
		t.Fatalf("Failed to parse received message: %v", err)
	}

	// Verify message content
	if receivedMsg.Type != testMsg.Type {
		t.Errorf("Expected type %s, got %s", testMsg.Type, receivedMsg.Type)
	}

	if receivedMsg.Topic != testMsg.Topic {
		t.Errorf("Expected topic %s, got %s", testMsg.Topic, receivedMsg.Topic)
	}

	if string(receivedMsg.Payload) != string(testMsg.Payload) {
		t.Errorf("Expected payload %s, got %s", string(testMsg.Payload), string(receivedMsg.Payload))
	}
}

func TestTCPClient_Send_InvalidJSON(t *testing.T) {
	// Create a mock transport
	mockTransport := &MockTransport{}

	// Create a pipe for testing
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewTCPClient(serverConn, mockTransport)

	// Create message with invalid JSON in RawMessage
	invalidMsg := proto.Message{
		Type:  "test",
		Topic: "test/topic",
		// This should cause json.Marshal to fail when marshaling the entire message
		Payload: json.RawMessage("invalid json"),
	}

	// This should fail because json.Marshal validates RawMessage content
	err := client.Send(invalidMsg)
	if err == nil {
		t.Error("Expected error when sending message with invalid JSON payload")
	}
}

func TestTCPClient_Send_ConnectionError(t *testing.T) {
	// Create a mock transport
	mockTransport := &MockTransport{}

	// Create a pipe and close client side to simulate connection error
	serverConn, clientConn := net.Pipe()
	clientConn.Close()
	defer serverConn.Close()

	client := NewTCPClient(serverConn, mockTransport)

	// Create test message
	testMsg := proto.Message{
		Type:    "test",
		Topic:   "test/topic",
		Payload: json.RawMessage(`{"test": "data"}`),
	}

	// Send should fail due to closed connection
	err := client.Send(testMsg)
	if err == nil {
		t.Error("Expected error when sending to closed connection")
	}
}

func TestTCPClient_Meta(t *testing.T) {
	// Create a mock transport
	mockTransport := &MockTransport{}

	// Create a pipe for testing
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	client := NewTCPClient(serverConn, mockTransport)

	// Set some metadata
	client.DeviceMetadata.Name = "test-device"
	client.DeviceMetadata.Firmware = "1.0.0"
	client.DeviceMetadata.LastSeen = time.Now()

	meta := client.Meta()

	if meta == nil {
		t.Fatal("Expected metadata to be returned")
	}

	if meta.Name != "test-device" {
		t.Errorf("Expected name 'test-device', got %s", meta.Name)
	}

	if meta.Firmware != "1.0.0" {
		t.Errorf("Expected firmware '1.0.0', got %s", meta.Firmware)
	}

	if meta.Transport != mockTransport {
		t.Error("Expected transport to be set")
	}

	// Verify it returns a pointer to the same object
	if meta != &client.DeviceMetadata {
		t.Error("Expected Meta() to return pointer to DeviceMetadata")
	}
}

// MockTransport for testing
type MockTransport struct {
	onMessage    func(proto.Message)
	onConnect    func(Client) error
	onDisconnect func(Client)
}

func (m *MockTransport) Start() error {
	return nil
}

func (m *MockTransport) OnMessage(fn func(proto.Message)) {
	m.onMessage = fn
}

func (m *MockTransport) OnConnect(fn func(Client) error) {
	m.onConnect = fn
}

func (m *MockTransport) OnDisconnect(fn func(Client)) {
	m.onDisconnect = fn
}

func (m *MockTransport) Shutdown() error {
	return nil
}

func (m *MockTransport) Meta() TransportMetadata {
	return TransportMetadata{
		ID:       "mock-transport",
		Name:     "Mock Transport",
		Protocol: "mock",
		Address:  "mock://localhost",
	}
}

func (m *MockTransport) SetName(name string) {}

func (m *MockTransport) SetDescription(description string) {}

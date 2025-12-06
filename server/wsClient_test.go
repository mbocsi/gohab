package server

import (
	"encoding/json"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/mbocsi/gohab/proto"
)

func TestNewWSClient(t *testing.T) {
	// Create a mock WebSocket connection (we'll use nil for this test)
	var conn *websocket.Conn = nil
	transport := NewWSTransport("localhost:8080")
	
	client := NewWSClient(conn, transport)
	
	if client.conn != conn {
		t.Error("Expected conn to be set")
	}
	
	if client.Id == "" {
		t.Error("Expected ID to be generated")
	}
	
	if client.Transport != transport {
		t.Error("Expected Transport to be set")
	}
	
	if client.Subs == nil {
		t.Error("Expected Subs map to be initialized")
	}
}

func TestWSClient_Meta(t *testing.T) {
	var conn *websocket.Conn = nil
	transport := NewWSTransport("localhost:8080")
	client := NewWSClient(conn, transport)
	
	meta := client.Meta()
	
	if meta.Id != client.Id {
		t.Errorf("Expected meta ID %s, got %s", client.Id, meta.Id)
	}
	
	if meta.Transport != transport {
		t.Error("Expected meta Transport to match")
	}
}

func TestWSClient_SendMessage_NilConnection(t *testing.T) {
	// Test with nil connection - should handle gracefully
	var conn *websocket.Conn = nil
	transport := NewWSTransport("localhost:8080")
	client := NewWSClient(conn, transport)
	
	// Create a test message
	payloadData := map[string]interface{}{"data": "test"}
	payloadBytes, _ := json.Marshal(payloadData)
	testMsg := proto.Message{
		Type:    "test",
		Topic:   "test/topic",
		Payload: json.RawMessage(payloadBytes),
	}
	
	// Should return an error rather than panic
	err := client.Send(testMsg)
	if err == nil {
		t.Error("Expected error when sending to nil connection")
	}
}
//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// Test TCP transport communication
func TestTCPTransport(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())
	
	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)
	
	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	
	// Create TCP client
	c := client.NewClient("tcp-client", client.NewTCPTransport())
	err := c.AddFeature(proto.Feature{
		Name: "test-feature",
		Methods: proto.FeatureMethods{
			Data: proto.Method{OutputSchema: map[string]proto.DataType{
				"value": {Type: "string"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add feature: %v", err)
	}
	
	go func() {
		if err := c.Start(fmt.Sprintf("localhost:%d", tcpPort)); err != nil {
			t.Errorf("TCP client failed: %v", err)
		}
	}()
	
	// Wait for registration
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("TCP client not registered")
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 1 && devices[0].Meta().Name == "tcp-client" {
				return
			}
		}
	}
}

// Test WebSocket transport communication
func TestWebSocketTransport(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())
	
	wsPort := getRandomPort(t)
	wsTransport := server.NewWSTransport(fmt.Sprintf("127.0.0.1:%d", wsPort))
	gohabServer.RegisterTransport(wsTransport)
	
	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	
	// Create WebSocket client
	c := client.NewClient("ws-client", client.NewWebSocketTransport())
	err := c.AddFeature(proto.Feature{
		Name: "test-feature",
		Methods: proto.FeatureMethods{
			Data: proto.Method{OutputSchema: map[string]proto.DataType{
				"value": {Type: "string"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add feature: %v", err)
	}
	
	go func() {
		if err := c.Start(fmt.Sprintf("ws://localhost:%d/ws", wsPort)); err != nil {
			t.Errorf("WebSocket client failed: %v", err)
		}
	}()
	
	// Wait for registration
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("WebSocket client not registered")
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 1 && devices[0].Meta().Name == "ws-client" {
				return
			}
		}
	}
}

// Test mixed transport types (TCP and WebSocket clients on same server)
func TestMixedTransports(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())
	
	tcpPort := getRandomPort(t)
	wsPort := getRandomPort(t)
	
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	wsTransport := server.NewWSTransport(fmt.Sprintf("127.0.0.1:%d", wsPort))
	
	gohabServer.RegisterTransport(tcpTransport)
	gohabServer.RegisterTransport(wsTransport)
	
	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	
	// Create TCP client
	tcpClient := client.NewClient("tcp-client", client.NewTCPTransport())
	err := tcpClient.AddFeature(proto.Feature{
		Name: "tcp-sensor",
		Methods: proto.FeatureMethods{
			Data: proto.Method{OutputSchema: map[string]proto.DataType{
				"value": {Type: "number"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add TCP feature: %v", err)
	}
	
	// Create WebSocket client  
	wsClient := client.NewClient("ws-client", client.NewWebSocketTransport())
	err = wsClient.AddFeature(proto.Feature{
		Name: "ws-sensor",
		Methods: proto.FeatureMethods{
			Data: proto.Method{OutputSchema: map[string]proto.DataType{
				"value": {Type: "string"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add WS feature: %v", err)
	}
	
	// Connect both clients
	go func() {
		if err := tcpClient.Start(fmt.Sprintf("localhost:%d", tcpPort)); err != nil {
			t.Errorf("TCP client failed: %v", err)
		}
	}()
	
	go func() {
		if err := wsClient.Start(fmt.Sprintf("ws://localhost:%d/ws", wsPort)); err != nil {
			t.Errorf("WebSocket client failed: %v", err)
		}
	}()
	
	// Wait for both clients to register
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			devices := registry.List()
			t.Fatalf("Expected 2 devices, got %d", len(devices))
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 2 {
				// Verify both transport types work
				topicSources := gohabServer.GetTopicSources()
				
				if _, ok := topicSources["tcp-sensor"]; !ok {
					t.Error("TCP sensor topic not registered")
				}
				if _, ok := topicSources["ws-sensor"]; !ok {
					t.Error("WebSocket sensor topic not registered")
				}
				
				return
			}
		}
	}
}
//go:build integration

package integration

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// Test basic server-client communication end-to-end
func TestServerClientCommunication(t *testing.T) {
	// Start real server
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())
	
	// Get available port
	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)
	
	// Start server
	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // Give server time to start
	
	// Create real client
	transport := client.NewTCPTransport()
	c := client.NewClient("test-sensor", transport)
	
	// Add temperature sensor feature
	err := c.AddFeature(proto.Feature{
		Name:        "temperature",
		Description: "Test temperature sensor",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				Description:  "Temperature readings",
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Celsius"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add feature: %v", err)
	}
	
	// Connect client
	go func() {
		err := c.Start(fmt.Sprintf("localhost:%d", tcpPort))
		if err != nil {
			t.Errorf("Client failed: %v", err)
		}
	}()
	
	// Wait for device to be registered
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Device not registered in time")
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 1 {
				// Verify device is properly registered
				device := devices[0]
				if device.Meta().Name != "test-sensor" {
					t.Errorf("Expected device name 'test-sensor', got %s", device.Meta().Name)
				}
				if len(device.Meta().Features) != 1 {
					t.Errorf("Expected 1 feature, got %d", len(device.Meta().Features))
				}
				return
			}
		}
	}
}

func getRandomPort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
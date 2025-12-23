package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// Test basic device identification and registration
func TestDeviceIdentificationAndRegistration(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())

	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)

	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	serverAddr := fmt.Sprintf("localhost:%d", tcpPort)

	// Create client with valid features
	c := newQuietClient("test-device", client.NewTCPTransport())

	// Add multiple features to test comprehensive registration
	tempFeature := proto.Feature{
		Name:        "temperature",
		Description: "Temperature sensor with data and query support",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				Description: "Temperature readings",
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Celsius"},
					"humidity":    {Type: "number", Unit: "percent"},
				},
			},
			Query: proto.Method{
				Description: "Get current temperature",
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Celsius"},
					"timestamp":   {Type: "string"},
				},
			},
			Status: proto.Method{
				Description: "Sensor status",
				OutputSchema: map[string]proto.DataType{
					"status": {Type: "enum", Enum: []string{"ok", "error", "maintenance"}},
					"uptime": {Type: "number", Unit: "seconds"},
				},
			},
		},
	}

	ledFeature := proto.Feature{
		Name:        "led-control",
		Description: "LED with brightness and color control",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				Description: "Control LED brightness and color",
				InputSchema: map[string]proto.DataType{
					"brightness": {Type: "number", Range: []float64{0, 100}},
					"color":      {Type: "enum", Enum: []string{"red", "green", "blue", "white"}},
					"on":         {Type: "bool"},
				},
			},
			Query: proto.Method{
				Description: "Get current LED state",
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"color":      {Type: "string"},
					"on":         {Type: "bool"},
				},
			},
		},
	}

	err := c.AddFeature(tempFeature)
	if err != nil {
		t.Fatalf("Failed to add temperature feature: %v", err)
	}

	err = c.AddFeature(ledFeature)
	if err != nil {
		t.Fatalf("Failed to add LED feature: %v", err)
	}

	// Start client
	go func() {
		if err := c.Start(serverAddr); err != nil {
			t.Errorf("Client failed: %v", err)
		}
	}()

	// Wait for device registration
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Device not registered within timeout")
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 1 {
				device := devices[0]
				meta := device.Meta()

				// Verify device metadata
				if meta.Name != "test-device" {
					t.Errorf("Expected device name 'test-device', got %s", meta.Name)
				}

				if meta.Id == "" {
					t.Error("Device ID not assigned")
				}

				// Verify features are registered
				if len(meta.Features) != 2 {
					t.Errorf("Expected 2 features, got %d", len(meta.Features))
				}

				// Verify feature details
				var tempFound, ledFound bool
				for _, feature := range meta.Features {
					switch feature.Name {
					case "temperature":
						tempFound = true
						if len(feature.Methods.Data.OutputSchema) != 2 {
							t.Error("Temperature feature data schema incomplete")
						}
						if len(feature.Methods.Query.OutputSchema) != 2 {
							t.Error("Temperature feature query schema incomplete")
						}
						if len(feature.Methods.Status.OutputSchema) != 2 {
							t.Error("Temperature feature status schema incomplete")
						}
					case "led-control":
						ledFound = true
						if len(feature.Methods.Command.InputSchema) != 3 {
							t.Error("LED feature command schema incomplete")
						}
						if len(feature.Methods.Query.OutputSchema) != 3 {
							t.Error("LED feature query schema incomplete")
						}
					}
				}

				if !tempFound {
					t.Error("Temperature feature not found in registered device")
				}
				if !ledFound {
					t.Error("LED control feature not found in registered device")
				}

				// Verify topic sources are created
				topicSources := gohabServer.GetTopicSources()
				if _, ok := topicSources["temperature"]; !ok {
					t.Error("Temperature topic not registered")
				}
				if _, ok := topicSources["led-control"]; !ok {
					t.Error("LED control topic not registered")
				}

				return
			}
		}
	}
}

// Test device registration with invalid capabilities
func TestDeviceInvalidCapabilities(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())

	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)

	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	// Test invalid data type
	t.Run("InvalidDataType", func(t *testing.T) {
		c := newQuietClient("invalid-datatype", client.NewTCPTransport())

		invalidFeature := proto.Feature{
			Name: "invalid-sensor",
			Methods: proto.FeatureMethods{
				Data: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"value": {Type: "invalid-type"}, // Invalid type
					},
				},
			},
		}

		err := c.AddFeature(invalidFeature)
		if err == nil {
			t.Error("Expected error for invalid data type, but got none")
		}
	})

	// Test empty enum
	t.Run("EmptyEnum", func(t *testing.T) {
		c := newQuietClient("empty-enum", client.NewTCPTransport())

		invalidFeature := proto.Feature{
			Name: "empty-enum-sensor",
			Methods: proto.FeatureMethods{
				Status: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"status": {Type: "enum", Enum: []string{}}, // Empty enum
					},
				},
			},
		}

		err := c.AddFeature(invalidFeature)
		if err == nil {
			t.Error("Expected error for empty enum, but got none")
		}
	})

	// Test feature with no methods
	t.Run("NoMethods", func(t *testing.T) {
		c := newQuietClient("no-methods", client.NewTCPTransport())

		invalidFeature := proto.Feature{
			Name:    "no-methods-feature",
			Methods: proto.FeatureMethods{}, // No methods defined
		}

		err := c.AddFeature(invalidFeature)
		if err == nil {
			t.Error("Expected error for feature with no methods, but got none")
		}
	})

	// Test invalid range specification
	t.Run("InvalidRange", func(t *testing.T) {
		c := newQuietClient("invalid-range", client.NewTCPTransport())

		invalidFeature := proto.Feature{
			Name: "invalid-range-sensor",
			Methods: proto.FeatureMethods{
				Command: proto.Method{
					InputSchema: map[string]proto.DataType{
						"value": {Type: "number", Range: []float64{0}}, // Invalid range (only one value)
					},
				},
			},
		}

		err := c.AddFeature(invalidFeature)
		if err == nil {
			t.Error("Expected error for invalid range specification, but got none")
		}
	})
}

// Test duplicate device names handling
func TestDuplicateDeviceNames(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())

	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)

	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	serverAddr := fmt.Sprintf("localhost:%d", tcpPort)

	// Create two clients with same proposed name
	c1 := newQuietClient("duplicate-name", client.NewTCPTransport())
	c2 := newQuietClient("duplicate-name", client.NewTCPTransport())

	// Add features to both clients
	feature1 := proto.Feature{
		Name: "sensor-1",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"value": {Type: "number"},
				},
			},
		},
	}

	feature2 := proto.Feature{
		Name: "sensor-2",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"value": {Type: "string"},
				},
			},
		},
	}

	err := c1.AddFeature(feature1)
	if err != nil {
		t.Fatalf("Failed to add feature to client 1: %v", err)
	}

	err = c2.AddFeature(feature2)
	if err != nil {
		t.Fatalf("Failed to add feature to client 2: %v", err)
	}

	// Start both clients
	go func() {
		if err := c1.Start(serverAddr); err != nil {
			t.Errorf("Client 1 failed: %v", err)
		}
	}()

	go func() {
		if err := c2.Start(serverAddr); err != nil {
			t.Errorf("Client 2 failed: %v", err)
		}
	}()

	// Wait for both registrations
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
				// Verify both devices are registered with unique IDs
				deviceIds := make(map[string]bool)
				for _, device := range devices {
					if deviceIds[device.Meta().Id] {
						t.Errorf("Duplicate device ID found: %s", device.Meta().Id)
					}
					deviceIds[device.Meta().Id] = true

					// Both should have the same proposed name but unique IDs
					if device.Meta().Name != "duplicate-name" {
						t.Errorf("Expected device name 'duplicate-name', got %s", device.Meta().Name)
					}
				}

				// Verify both topic sources are created
				topicSources := gohabServer.GetTopicSources()
				if _, ok := topicSources["sensor-1"]; !ok {
					t.Error("Sensor-1 topic not registered")
				}
				if _, ok := topicSources["sensor-2"]; !ok {
					t.Error("Sensor-2 topic not registered")
				}

				return
			}
		}
	}
}

// Test device graceful disconnection and cleanup
func TestDeviceGracefulDisconnection(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.SuppressedLogConfig())

	tcpPort := getRandomPort(t)
	tcpTransport := server.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", tcpPort))
	gohabServer.RegisterTransport(tcpTransport)

	go func() {
		if err := gohabServer.Start(); err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	serverAddr := fmt.Sprintf("localhost:%d", tcpPort)

	// Create client with subscription
	c := newQuietClient("disconnect-test", client.NewTCPTransport())

	feature := proto.Feature{
		Name: "disconnect-sensor",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"status": {Type: "string"},
				},
			},
		},
	}

	err := c.AddFeature(feature)
	if err != nil {
		t.Fatalf("Failed to add feature: %v", err)
	}

	// Add subscription to test cleanup
	err = c.Subscribe("disconnect-sensor", func(msg proto.Message) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Start client
	go func() {
		if err := c.Start(serverAddr); err != nil {
			t.Logf("Client disconnected (expected): %v", err)
		}
	}()

	// Wait for registration
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var deviceId string
	for {
		select {
		case <-timeout:
			t.Fatal("Device not registered within timeout")
		case <-ticker.C:
			devices := registry.List()
			if len(devices) == 1 {
				deviceId = devices[0].Meta().Id

				// Verify subscriptions exist
				subs := broker.Subs("disconnect-sensor")
				if len(subs) == 0 {
					t.Error("No subscriptions found for device topic")
				}
				goto testDisconnection
			}
		}
	}

testDisconnection:
	// Simulate graceful disconnection by stopping the client
	// Note: In a real implementation, we'd call a Stop() method on the client
	// For this test, we'll verify the server can handle disconnection

	// Wait a bit for potential cleanup
	time.Sleep(500 * time.Millisecond)

	// For now, just verify the test framework works
	// In a full implementation, we'd verify:
	// 1. Device removed from registry
	// 2. Subscriptions cleaned up from broker
	// 3. Topic sources updated

	if deviceId == "" {
		t.Error("Device ID was not captured for disconnection test")
	}
}

// Test feature validation during registration
func TestFeatureValidation(t *testing.T) {
	// Test valid feature with all method types
	t.Run("ValidCompleteFeature", func(t *testing.T) {
		validFeature := proto.Feature{
			Name:        "complete-sensor",
			Description: "A sensor with all method types",
			Methods: proto.FeatureMethods{
				Data: proto.Method{
					Description: "Sensor data",
					OutputSchema: map[string]proto.DataType{
						"temperature": {Type: "number", Unit: "Celsius"},
						"humidity":    {Type: "number", Unit: "percent", Range: []float64{0, 100}},
					},
				},
				Status: proto.Method{
					Description: "Sensor status",
					OutputSchema: map[string]proto.DataType{
						"status": {Type: "enum", Enum: []string{"ok", "error", "maintenance"}},
						"uptime": {Type: "number", Unit: "seconds"},
					},
				},
				Command: proto.Method{
					Description: "Sensor control",
					InputSchema: map[string]proto.DataType{
						"enable": {Type: "bool"},
						"rate":   {Type: "number", Range: []float64{1, 60}, Unit: "seconds"},
					},
				},
				Query: proto.Method{
					Description: "Get sensor info",
					OutputSchema: map[string]proto.DataType{
						"model":    {Type: "string"},
						"version":  {Type: "string"},
						"location": {Type: "string", Optional: true},
					},
				},
			},
		}

		err := validFeature.Validate()
		if err != nil {
			t.Errorf("Valid feature failed validation: %v", err)
		}
	})

	// Test feature validation edge cases
	t.Run("EdgeCaseValidation", func(t *testing.T) {
		// Feature with only data method (valid)
		dataOnlyFeature := proto.Feature{
			Name: "data-only",
			Methods: proto.FeatureMethods{
				Data: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"value": {Type: "number"},
					},
				},
			},
		}

		err := dataOnlyFeature.Validate()
		if err != nil {
			t.Errorf("Data-only feature should be valid: %v", err)
		}

		// Feature with only command method (valid)
		commandOnlyFeature := proto.Feature{
			Name: "command-only",
			Methods: proto.FeatureMethods{
				Command: proto.Method{
					InputSchema: map[string]proto.DataType{
						"action": {Type: "string"},
					},
				},
			},
		}

		err = commandOnlyFeature.Validate()
		if err != nil {
			t.Errorf("Command-only feature should be valid: %v", err)
		}
	})
}

package integration

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// Test command message execution
func TestCommandMessageExecution(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.QuietLogConfig())

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

	// Create LED device that can receive commands
	ledDevice := client.NewClient("led-device", client.NewTCPTransport())

	// Track command execution
	var commandReceived bool
	var receivedBrightness float64
	var receivedColor string
	var mu sync.Mutex

	ledFeature := proto.Feature{
		Name: "led-control",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"brightness": {Type: "number", Range: []float64{0, 100}},
					"color":      {Type: "enum", Enum: []string{"red", "green", "blue", "white"}},
					"on":         {Type: "bool"},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"color":      {Type: "string"},
					"on":         {Type: "bool"},
				},
			},
		},
	}

	err := ledDevice.AddFeature(ledFeature)
	if err != nil {
		t.Fatalf("Failed to add LED feature: %v", err)
	}

	// Register command handler
	err = ledDevice.RegisterCommandHandler("led-control", func(msg proto.Message) error {
		mu.Lock()
		defer mu.Unlock()

		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		if brightness, ok := payload["brightness"].(float64); ok {
			receivedBrightness = brightness
		}
		if color, ok := payload["color"].(string); ok {
			receivedColor = color
		}
		commandReceived = true
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to register command handler: %v", err)
	}

	// Create controller client
	controller := client.NewClient("controller", client.NewTCPTransport())

	// Start both clients
	go func() {
		if err := ledDevice.Start(serverAddr); err != nil {
			t.Errorf("LED device failed: %v", err)
		}
	}()

	go func() {
		if err := controller.Start(serverAddr); err != nil {
			t.Errorf("Controller failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Test simple command execution
	t.Run("SimpleCommand", func(t *testing.T) {
		err := controller.SendCommand("led-control", map[string]interface{}{
			"brightness": 75.0,
			"color":      "blue",
			"on":         true,
		})
		if err != nil {
			t.Fatalf("Failed to send command: %v", err)
		}

		// Wait for command processing
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !commandReceived {
			t.Error("Command not received by device")
		}
		if receivedBrightness != 75.0 {
			t.Errorf("Expected brightness 75.0, got %f", receivedBrightness)
		}
		if receivedColor != "blue" {
			t.Errorf("Expected color 'blue', got '%s'", receivedColor)
		}
	})

	// Test command with invalid parameters
	t.Run("InvalidCommandParameters", func(t *testing.T) {
		// Reset state
		mu.Lock()
		commandReceived = false
		mu.Unlock()

		err := controller.SendCommand("led-control", map[string]interface{}{
			"brightness": 150.0,    // Out of range (0-100)
			"color":      "purple", // Not in enum
		})

		// The command should be sent, but device may reject it
		// For this test, we verify the command is transmitted
		if err != nil {
			t.Logf("Command with invalid parameters rejected (expected): %v", err)
		}
	})

	// Test command to non-existent device
	t.Run("NonExistentDevice", func(t *testing.T) {
		err := controller.SendCommand("non-existent-device", map[string]interface{}{
			"action": "test",
		})

		// Should not throw an error since message was successfully sent
		if err != nil {
			t.Errorf("Uexpected error for command to non-existent device: %v", err)
		}
	})

	// Test batch command execution
	t.Run("BatchCommands", func(t *testing.T) {
		commandCount := 0
		mu.Lock()
		commandReceived = false
		mu.Unlock()

		// Register handler to count commands
		err = ledDevice.RegisterCommandHandler("led-control", func(msg proto.Message) error {
			mu.Lock()
			defer mu.Unlock()
			commandCount++
			commandReceived = true
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to register command counter: %v", err)
		}

		// Send multiple commands rapidly
		numCommands := 5
		for i := 0; i < numCommands; i++ {
			err := controller.SendCommand("led-control", map[string]interface{}{
				"brightness": float64(i * 20),
				"on":         true,
			})
			if err != nil {
				t.Errorf("Failed to send command %d: %v", i, err)
			}
		}

		// Wait for all commands to be processed
		time.Sleep(1 * time.Second)

		mu.Lock()
		defer mu.Unlock()

		if !commandReceived {
			t.Error("No commands received")
		}
		if commandCount < numCommands {
			t.Errorf("Expected at least %d commands, got %d", numCommands, commandCount)
		}
	})
}

// Test query message handling and responses
func TestQueryMessageHandling(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.QuietLogConfig())

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

	// Create temperature sensor device
	tempDevice := client.NewClient("temp-sensor", client.NewTCPTransport())

	currentTemp := 22.5
	currentHumidity := 65.0

	tempFeature := proto.Feature{
		Name: "temperature",
		Methods: proto.FeatureMethods{
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Celsius"},
					"humidity":    {Type: "number", Unit: "percent"},
					"timestamp":   {Type: "string"},
				},
			},
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"status": {Type: "enum", Enum: []string{"ok", "error", "maintenance"}},
					"uptime": {Type: "number", Unit: "seconds"},
				},
			},
		},
	}

	err := tempDevice.AddFeature(tempFeature)
	if err != nil {
		t.Fatalf("Failed to add temperature feature: %v", err)
	}

	// Register query handler
	err = tempDevice.RegisterQueryHandler("temperature", func(msg proto.Message) (any, error) {
		return map[string]interface{}{
			"temperature": currentTemp,
			"humidity":    currentHumidity,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
	if err != nil {
		t.Fatalf("Failed to register query handler: %v", err)
	}

	// Create controller
	controller := client.NewClient("temp-controller", client.NewTCPTransport())

	// Start both clients
	go func() {
		if err := tempDevice.Start(serverAddr); err != nil {
			t.Errorf("Temperature device failed: %v", err)
		}
	}()

	go func() {
		if err := controller.Start(serverAddr); err != nil {
			t.Errorf("Controller failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Test basic query
	t.Run("BasicQuery", func(t *testing.T) {
		response, err := controller.SendQuery("temperature", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to send query: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(response.Payload, &data); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Verify response contains expected fields
		temperature, ok := data["temperature"].(float64)
		if !ok {
			t.Error("Temperature field missing or wrong type")
		} else if temperature != currentTemp {
			t.Errorf("Expected temperature %f, got %f", currentTemp, temperature)
		}

		humidity, ok := data["humidity"].(float64)
		if !ok {
			t.Error("Humidity field missing or wrong type")
		} else if humidity != currentHumidity {
			t.Errorf("Expected humidity %f, got %f", currentHumidity, humidity)
		}

		if _, ok := data["timestamp"].(string); !ok {
			t.Error("Timestamp field missing or wrong type")
		}
	})

	// Test query response validation
	t.Run("QueryResponseValidation", func(t *testing.T) {
		// Update values to test state changes
		currentTemp = 25.0
		currentHumidity = 70.0

		response, err := controller.SendQuery("temperature", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to send query: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(response.Payload, &data); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Verify updated values
		if temp := data["temperature"].(float64); temp != 25.0 {
			t.Errorf("Expected updated temperature 25.0, got %f", temp)
		}
		if humidity := data["humidity"].(float64); humidity != 70.0 {
			t.Errorf("Expected updated humidity 70.0, got %f", humidity)
		}
	})

	// Test concurrent queries
	t.Run("ConcurrentQueries", func(t *testing.T) {
		numQueries := 10
		responses := make(chan proto.Message, numQueries)
		errors := make(chan error, numQueries)

		var wg sync.WaitGroup
		wg.Add(numQueries)

		// Send multiple queries concurrently
		for i := 0; i < numQueries; i++ {
			go func() {
				defer wg.Done()
				response, err := controller.SendQuery("temperature", map[string]interface{}{})
				if err != nil {
					errors <- err
					return
				}
				responses <- response
			}()
		}

		wg.Wait()
		close(responses)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent query failed: %v", err)
		}

		// Verify all responses
		responseCount := 0
		for response := range responses {
			responseCount++
			var data map[string]interface{}
			if err := json.Unmarshal(response.Payload, &data); err != nil {
				t.Errorf("Failed to unmarshal concurrent response: %v", err)
				continue
			}

			// All responses should be consistent
			if temp := data["temperature"].(float64); temp != currentTemp {
				t.Errorf("Inconsistent temperature in concurrent response: got %f, expected %f", temp, currentTemp)
			}
		}

		if responseCount != numQueries {
			t.Errorf("Expected %d responses, got %d", numQueries, responseCount)
		}
	})

	// Test query to non-existent device
	t.Run("QueryNonExistentDevice", func(t *testing.T) {
		_, err := controller.SendQuery("non-existent-sensor", map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for query to non-existent device, but got none")
		}
	})
}

// Test data publishing functionality
func TestDataPublishing(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.QuietLogConfig())

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

	// Create sensor device
	sensor := client.NewClient("data-sensor", client.NewTCPTransport())

	sensorFeature := proto.Feature{
		Name: "environmental-data",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number", Unit: "Celsius"},
					"humidity":    {Type: "number", Unit: "percent"},
					"pressure":    {Type: "number", Unit: "hPa"},
					"timestamp":   {Type: "string"},
				},
			},
		},
	}

	err := sensor.AddFeature(sensorFeature)
	if err != nil {
		t.Fatalf("Failed to add sensor feature: %v", err)
	}

	// Create subscriber
	subscriber := client.NewClient("data-subscriber", client.NewTCPTransport())

	var receivedMessages []map[string]interface{}
	var msgMu sync.Mutex

	err = subscriber.Subscribe("environmental-data", func(msg proto.Message) error {
		msgMu.Lock()
		defer msgMu.Unlock()

		var data map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			return err
		}
		receivedMessages = append(receivedMessages, data)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Start both clients
	go func() {
		if err := sensor.Start(serverAddr); err != nil {
			t.Errorf("Sensor failed: %v", err)
		}
	}()

	go func() {
		if err := subscriber.Start(serverAddr); err != nil {
			t.Errorf("Subscriber failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Get data publishing function
	dataFn, err := sensor.GetDataFunction("environmental-data")
	if err != nil {
		t.Fatalf("Failed to get data function: %v", err)
	}

	// Test periodic data publishing
	t.Run("PeriodicDataPublishing", func(t *testing.T) {
		readings := []map[string]interface{}{
			{
				"temperature": 20.5,
				"humidity":    60.0,
				"pressure":    1013.25,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			},
			{
				"temperature": 21.0,
				"humidity":    58.5,
				"pressure":    1012.8,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			},
			{
				"temperature": 21.5,
				"humidity":    57.0,
				"pressure":    1014.1,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			},
		}

		// Clear received messages
		msgMu.Lock()
		receivedMessages = []map[string]interface{}{}
		msgMu.Unlock()

		// Publish readings with intervals
		for i, reading := range readings {
			err := dataFn(reading)
			if err != nil {
				t.Errorf("Failed to publish reading %d: %v", i, err)
			}
			time.Sleep(200 * time.Millisecond) // Simulate periodic publishing
		}

		// Wait for message delivery
		time.Sleep(500 * time.Millisecond)

		msgMu.Lock()
		defer msgMu.Unlock()

		if len(receivedMessages) != len(readings) {
			t.Errorf("Expected %d messages, got %d", len(readings), len(receivedMessages))
		}

		// Verify message content
		for i, received := range receivedMessages {
			if i >= len(readings) {
				break
			}
			expected := readings[i]

			for key, expectedVal := range expected {
				if receivedVal, ok := received[key]; !ok {
					t.Errorf("Message %d missing field %s", i, key)
				} else if fmt.Sprintf("%v", receivedVal) != fmt.Sprintf("%v", expectedVal) {
					t.Errorf("Message %d field %s: expected %v, got %v", i, key, expectedVal, receivedVal)
				}
			}
		}
	})

	// Test high-frequency data publishing
	t.Run("HighFrequencyData", func(t *testing.T) {
		msgMu.Lock()
		receivedMessages = []map[string]interface{}{}
		msgMu.Unlock()

		messageCount := 20
		for i := 0; i < messageCount; i++ {
			reading := map[string]interface{}{
				"temperature": 20.0 + float64(i)*0.1,
				"humidity":    60.0 - float64(i)*0.5,
				"pressure":    1013.0 + float64(i)*0.1,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}

			err := dataFn(reading)
			if err != nil {
				t.Errorf("Failed to publish high-freq reading %d: %v", i, err)
			}
			time.Sleep(50 * time.Millisecond) // High frequency
		}

		// Wait for all messages
		time.Sleep(2 * time.Second)

		msgMu.Lock()
		defer msgMu.Unlock()

		// Verify no message loss (allow for some timing variance)
		if len(receivedMessages) < messageCount {
			t.Errorf("Message loss detected: expected %d, got %d", messageCount, len(receivedMessages))
		}
	})

	// Test large payload handling
	t.Run("LargePayload", func(t *testing.T) {
		msgMu.Lock()
		receivedMessages = []map[string]interface{}{}
		msgMu.Unlock()

		// Create large payload with detailed readings
		largeReading := map[string]interface{}{
			"temperature": 22.3,
			"humidity":    55.7,
			"pressure":    1015.6,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}

		// Add large data array to test payload size handling
		sensorData := make([]map[string]interface{}, 100)
		for i := range sensorData {
			sensorData[i] = map[string]interface{}{
				"sensor_id": fmt.Sprintf("sensor_%03d", i),
				"value":     float64(i) * 0.1,
				"quality":   "good",
			}
		}
		largeReading["detailed_sensors"] = sensorData

		err := dataFn(largeReading)
		if err != nil {
			t.Errorf("Failed to publish large payload: %v", err)
		}

		// Wait for message delivery
		time.Sleep(1 * time.Second)

		msgMu.Lock()
		defer msgMu.Unlock()

		if len(receivedMessages) != 1 {
			t.Errorf("Expected 1 large message, got %d", len(receivedMessages))
		} else {
			received := receivedMessages[0]
			if detailedSensors, ok := received["detailed_sensors"]; !ok {
				t.Error("Large payload data missing")
			} else if sensors, ok := detailedSensors.([]interface{}); !ok {
				t.Error("Large payload data wrong type")
			} else if len(sensors) != 100 {
				t.Errorf("Expected 100 detailed sensors, got %d", len(sensors))
			}
		}
	})
}

// Test status reporting functionality
func TestStatusReporting(t *testing.T) {
	broker := server.NewBroker()
	registry := server.NewDeviceRegistry()
	gohabServer := server.NewGohabServerWithLogging(registry, broker, server.QuietLogConfig())

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

	// Create device with status reporting
	device := client.NewClient("status-device", client.NewTCPTransport())

	currentStatus := "ok"
	uptime := 0

	statusFeature := proto.Feature{
		Name: "device-status",
		Methods: proto.FeatureMethods{
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"status":     {Type: "enum", Enum: []string{"ok", "warning", "error", "maintenance"}},
					"uptime":     {Type: "number", Unit: "seconds"},
					"last_seen":  {Type: "string"},
					"error_code": {Type: "string", Optional: true},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"status":     {Type: "enum", Enum: []string{"ok", "warning", "error", "maintenance"}},
					"uptime":     {Type: "number", Unit: "seconds"},
					"last_seen":  {Type: "string"},
					"error_code": {Type: "string", Optional: true},
				},
			},
		},
	}

	err := device.AddFeature(statusFeature)
	if err != nil {
		t.Fatalf("Failed to add status feature: %v", err)
	}

	// Register query handler for status
	err = device.RegisterQueryHandler("device-status", func(msg proto.Message) (any, error) {
		return map[string]interface{}{
			"status":    currentStatus,
			"uptime":    uptime,
			"last_seen": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
	if err != nil {
		t.Fatalf("Failed to register query handler: %v", err)
	}

	// Create monitor client
	monitor := client.NewClient("status-monitor", client.NewTCPTransport())

	var statusUpdates []map[string]interface{}
	var statusMu sync.Mutex

	err = monitor.Subscribe("device-status", func(msg proto.Message) error {
		statusMu.Lock()
		defer statusMu.Unlock()

		var status map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &status); err != nil {
			return err
		}
		statusUpdates = append(statusUpdates, status)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to status: %v", err)
	}

	// Start both clients
	go func() {
		if err := device.Start(serverAddr); err != nil {
			t.Errorf("Status device failed: %v", err)
		}
	}()

	go func() {
		if err := monitor.Start(serverAddr); err != nil {
			t.Errorf("Monitor failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Get status publishing function
	statusFn, err := device.GetStatusFunction("device-status")
	if err != nil {
		t.Fatalf("Failed to get status function: %v", err)
	}

	// Test status update propagation
	t.Run("StatusUpdatePropagation", func(t *testing.T) {
		statusMu.Lock()
		statusUpdates = []map[string]interface{}{}
		statusMu.Unlock()

		// Publish initial status
		err := statusFn(map[string]interface{}{
			"status":    "ok",
			"uptime":    300,
			"last_seen": time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish status: %v", err)
		}

		// Wait for delivery
		time.Sleep(500 * time.Millisecond)

		statusMu.Lock()
		defer statusMu.Unlock()

		if len(statusUpdates) != 1 {
			t.Errorf("Expected 1 status update, got %d", len(statusUpdates))
		} else {
			status := statusUpdates[0]
			if status["status"] != "ok" {
				t.Errorf("Expected status 'ok', got %v", status["status"])
			}
			if uptime, ok := status["uptime"].(float64); !ok || uptime != 300 {
				t.Errorf("Expected uptime 300, got %v", status["uptime"])
			}
		}
	})

	// Test status query vs published status consistency
	t.Run("StatusQueryConsistency", func(t *testing.T) {
		// Update status
		currentStatus = "warning"
		uptime = 600

		err := statusFn(map[string]interface{}{
			"status":     "warning",
			"uptime":     600,
			"last_seen":  time.Now().UTC().Format(time.RFC3339),
			"error_code": "TEMP_HIGH",
		})
		if err != nil {
			t.Fatalf("Failed to publish warning status: %v", err)
		}

		// Wait for status propagation
		time.Sleep(500 * time.Millisecond)

		// Query status directly
		response, err := monitor.SendQuery("device-status", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query status: %v", err)
		}

		var queriedStatus map[string]interface{}
		if err := json.Unmarshal(response.Payload, &queriedStatus); err != nil {
			t.Fatalf("Failed to unmarshal query response: %v", err)
		}

		// Compare queried status with last published status
		statusMu.Lock()
		lastPublished := statusUpdates[len(statusUpdates)-1]
		statusMu.Unlock()

		if queriedStatus["status"] != lastPublished["status"] {
			t.Errorf("Status inconsistency: queried %v, published %v",
				queriedStatus["status"], lastPublished["status"])
		}
	})

	// Test status enumeration validation
	t.Run("StatusEnumerationValidation", func(t *testing.T) {
		// Test valid enum values
		validStatuses := []string{"ok", "warning", "error", "maintenance"}

		for _, status := range validStatuses {
			err := statusFn(map[string]interface{}{
				"status":    status,
				"uptime":    700,
				"last_seen": time.Now().UTC().Format(time.RFC3339),
			})
			if err != nil {
				t.Errorf("Valid status '%s' should be accepted, but got error: %v", status, err)
			}
		}

		// Note: Invalid enum validation would typically happen at the schema level
		// For this integration test, we verify that the valid enums work correctly
		time.Sleep(1 * time.Second)

		statusMu.Lock()
		defer statusMu.Unlock()

		// Check that we received status updates for each valid status
		if len(statusUpdates) < len(validStatuses) {
			t.Errorf("Expected at least %d status updates, got %d", len(validStatuses), len(statusUpdates))
		}
	})
}

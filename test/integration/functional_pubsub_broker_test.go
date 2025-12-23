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

// Test subscription management lifecycle
func TestSubscriptionManagement(t *testing.T) {
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

	// Create publisher
	publisher := newQuietClient("test-publisher", client.NewTCPTransport())
	err := publisher.AddFeature(proto.Feature{
		Name: "test-topic",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"message": {Type: "string"},
					"count":   {Type: "number"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add publisher feature: %v", err)
	}

	// Create subscriber
	subscriber := newQuietClient("test-subscriber", client.NewTCPTransport())

	var receivedMessages []map[string]interface{}
	var msgMu sync.Mutex

	err = subscriber.Subscribe("test-topic", func(msg proto.Message) error {
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
		if err := publisher.Start(serverAddr); err != nil {
			t.Errorf("Publisher failed: %v", err)
		}
	}()

	go func() {
		if err := subscriber.Start(serverAddr); err != nil {
			t.Errorf("Subscriber failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Test basic subscription setup
	t.Run("BasicSubscriptionSetup", func(t *testing.T) {
		// Verify subscriber is registered for the topic
		subs := broker.Subs("test-topic")
		if len(subs) != 1 {
			t.Errorf("Expected 1 subscriber, got %d", len(subs))
		}
	})

	// Test message delivery to subscriber
	t.Run("MessageDeliveryToSubscriber", func(t *testing.T) {
		msgMu.Lock()
		receivedMessages = []map[string]interface{}{}
		msgMu.Unlock()

		// Get data function and publish
		dataFn, err := publisher.GetDataFunction("test-topic")
		if err != nil {
			t.Fatalf("Failed to get data function: %v", err)
		}

		testMessage := map[string]interface{}{
			"message": "Hello subscriber!",
			"count":   1,
		}

		err = dataFn(testMessage)
		if err != nil {
			t.Fatalf("Failed to publish message: %v", err)
		}

		// Wait for delivery
		time.Sleep(500 * time.Millisecond)

		msgMu.Lock()
		defer msgMu.Unlock()

		if len(receivedMessages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(receivedMessages))
		} else {
			received := receivedMessages[0]
			if received["message"] != "Hello subscriber!" {
				t.Errorf("Expected message 'Hello subscriber!', got %v", received["message"])
			}
			if count, ok := received["count"].(float64); !ok || count != 1 {
				t.Errorf("Expected count 1, got %v", received["count"])
			}
		}
	})

	// Test subscription to non-existent topic
	t.Run("SubscriptionToNonExistentTopic", func(t *testing.T) {
		newSubscriber := newQuietClient("new-subscriber", client.NewTCPTransport())

		var nonExistentMessages []map[string]interface{}
		var nonExistentMu sync.Mutex

		err := newSubscriber.Subscribe("non-existent-topic", func(msg proto.Message) error {
			nonExistentMu.Lock()
			defer nonExistentMu.Unlock()

			var data map[string]interface{}
			if err := json.Unmarshal(msg.Payload, &data); err != nil {
				return err
			}
			nonExistentMessages = append(nonExistentMessages, data)
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to subscribe to non-existent topic: %v", err)
		}

		go func() {
			if err := newSubscriber.Start(serverAddr); err != nil {
				t.Errorf("New subscriber failed: %v", err)
			}
		}()

		// Wait for connection
		time.Sleep(500 * time.Millisecond)

		// Verify subscription is registered even though topic doesn't exist yet
		subs := broker.Subs("non-existent-topic")
		if len(subs) != 1 {
			t.Errorf("Expected 1 subscriber for non-existent topic, got %d", len(subs))
		}

		// Verify no messages received (since no publisher)
		nonExistentMu.Lock()
		defer nonExistentMu.Unlock()

		if len(nonExistentMessages) != 0 {
			t.Errorf("Expected 0 messages for non-existent topic, got %d", len(nonExistentMessages))
		}
	})
}

// Test multiple subscribers per topic
func TestMultipleSubscribers(t *testing.T) {
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

	// Create publisher
	publisher := newQuietClient("multi-publisher", client.NewTCPTransport())
	err := publisher.AddFeature(proto.Feature{
		Name: "broadcast-topic",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"broadcast_id": {Type: "string"},
					"data":         {Type: "string"},
					"timestamp":    {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add broadcast feature: %v", err)
	}

	// Create multiple subscribers
	numSubscribers := 5
	subscribers := make([]*client.Client, numSubscribers)
	receivedCounts := make([]int, numSubscribers)
	countMutexes := make([]sync.Mutex, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		subscribers[i] = newQuietClient(fmt.Sprintf("subscriber-%d", i), client.NewTCPTransport())

		// Capture the index for the closure
		idx := i
		err := subscribers[i].Subscribe("broadcast-topic", func(msg proto.Message) error {
			countMutexes[idx].Lock()
			defer countMutexes[idx].Unlock()
			receivedCounts[idx]++
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to subscribe client %d: %v", i, err)
		}
	}

	// Start publisher
	go func() {
		if err := publisher.Start(serverAddr); err != nil {
			t.Errorf("Publisher failed: %v", err)
		}
	}()

	// Start all subscribers
	for i, sub := range subscribers {
		go func(idx int, client *client.Client) {
			if err := client.Start(serverAddr); err != nil {
				t.Errorf("Subscriber %d failed: %v", idx, err)
			}
		}(i, sub)
	}

	// Wait for all connections
	time.Sleep(2 * time.Second)

	// Test one-to-many broadcasting
	t.Run("OneToManyBroadcasting", func(t *testing.T) {
		// Reset counters
		for i := range receivedCounts {
			countMutexes[i].Lock()
			receivedCounts[i] = 0
			countMutexes[i].Unlock()
		}

		// Verify all subscribers are registered
		subs := broker.Subs("broadcast-topic")
		if len(subs) != numSubscribers {
			t.Errorf("Expected %d subscribers, got %d", numSubscribers, len(subs))
		}

		// Get data function and broadcast messages
		dataFn, err := publisher.GetDataFunction("broadcast-topic")
		if err != nil {
			t.Fatalf("Failed to get data function: %v", err)
		}

		numMessages := 3
		for i := 0; i < numMessages; i++ {
			message := map[string]interface{}{
				"broadcast_id": fmt.Sprintf("msg-%d", i),
				"data":         fmt.Sprintf("Broadcast message %d", i),
				"timestamp":    time.Now().UTC().Format(time.RFC3339),
			}

			err := dataFn(message)
			if err != nil {
				t.Errorf("Failed to broadcast message %d: %v", i, err)
			}
			time.Sleep(100 * time.Millisecond) // Small delay between messages
		}

		// Wait for all deliveries
		time.Sleep(1 * time.Second)

		// Verify all subscribers received all messages
		for i := 0; i < numSubscribers; i++ {
			countMutexes[i].Lock()
			count := receivedCounts[i]
			countMutexes[i].Unlock()

			if count != numMessages {
				t.Errorf("Subscriber %d: expected %d messages, got %d", i, numMessages, count)
			}
		}
	})
}

// Test message routing and topic isolation
func TestMessageRouting(t *testing.T) {
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

	// Create publishers for different topics
	tempPublisher := newQuietClient("temp-publisher", client.NewTCPTransport())
	err := tempPublisher.AddFeature(proto.Feature{
		Name: "temperature",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number"},
					"location":    {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add temperature feature: %v", err)
	}

	lightPublisher := newQuietClient("light-publisher", client.NewTCPTransport())
	err = lightPublisher.AddFeature(proto.Feature{
		Name: "lighting",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"room":       {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add lighting feature: %v", err)
	}

	// Create specialized subscribers
	tempSubscriber := newQuietClient("temp-monitor", client.NewTCPTransport())
	lightSubscriber := newQuietClient("light-monitor", client.NewTCPTransport())

	var tempMessages []map[string]interface{}
	var lightMessages []map[string]interface{}
	var tempMu sync.Mutex
	var lightMu sync.Mutex

	err = tempSubscriber.Subscribe("temperature", func(msg proto.Message) error {
		tempMu.Lock()
		defer tempMu.Unlock()

		var data map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			return err
		}
		tempMessages = append(tempMessages, data)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to temperature: %v", err)
	}

	err = lightSubscriber.Subscribe("lighting", func(msg proto.Message) error {
		lightMu.Lock()
		defer lightMu.Unlock()

		var data map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			return err
		}
		lightMessages = append(lightMessages, data)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to lighting: %v", err)
	}

	// Start all clients
	clients := []*client.Client{tempPublisher, lightPublisher, tempSubscriber, lightSubscriber}
	for i, c := range clients {
		go func(idx int, client *client.Client) {
			if err := client.Start(serverAddr); err != nil {
				t.Errorf("Client %d failed: %v", idx, err)
			}
		}(i, c)
	}

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Test topic isolation
	t.Run("TopicIsolation", func(t *testing.T) {
		tempMu.Lock()
		lightMu.Lock()
		tempMessages = []map[string]interface{}{}
		lightMessages = []map[string]interface{}{}
		tempMu.Unlock()
		lightMu.Unlock()

		// Get publishing functions
		tempDataFn, err := tempPublisher.GetDataFunction("temperature")
		if err != nil {
			t.Fatalf("Failed to get temperature data function: %v", err)
		}

		lightDataFn, err := lightPublisher.GetDataFunction("lighting")
		if err != nil {
			t.Fatalf("Failed to get lighting data function: %v", err)
		}

		// Publish temperature data
		err = tempDataFn(map[string]interface{}{
			"temperature": 23.5,
			"location":    "living-room",
		})
		if err != nil {
			t.Fatalf("Failed to publish temperature: %v", err)
		}

		// Publish lighting data
		err = lightDataFn(map[string]interface{}{
			"brightness": 75,
			"room":       "bedroom",
		})
		if err != nil {
			t.Fatalf("Failed to publish lighting: %v", err)
		}

		// Wait for delivery
		time.Sleep(500 * time.Millisecond)

		// Verify topic isolation
		tempMu.Lock()
		lightMu.Lock()
		defer tempMu.Unlock()
		defer lightMu.Unlock()

		if len(tempMessages) != 1 {
			t.Errorf("Expected 1 temperature message, got %d", len(tempMessages))
		}
		if len(lightMessages) != 1 {
			t.Errorf("Expected 1 lighting message, got %d", len(lightMessages))
		}

		// Verify correct message content
		if len(tempMessages) > 0 {
			if temp, ok := tempMessages[0]["temperature"].(float64); !ok || temp != 23.5 {
				t.Errorf("Expected temperature 23.5, got %v", tempMessages[0]["temperature"])
			}
			if location := tempMessages[0]["location"].(string); location != "living-room" {
				t.Errorf("Expected location 'living-room', got %s", location)
			}
		}

		if len(lightMessages) > 0 {
			if brightness, ok := lightMessages[0]["brightness"].(float64); !ok || brightness != 75 {
				t.Errorf("Expected brightness 75, got %v", lightMessages[0]["brightness"])
			}
			if room := lightMessages[0]["room"].(string); room != "bedroom" {
				t.Errorf("Expected room 'bedroom', got %s", room)
			}
		}
	})

	// Test message ordering
	t.Run("MessageOrdering", func(t *testing.T) {
		tempMu.Lock()
		tempMessages = []map[string]interface{}{}
		tempMu.Unlock()

		tempDataFn, err := tempPublisher.GetDataFunction("temperature")
		if err != nil {
			t.Fatalf("Failed to get temperature data function: %v", err)
		}

		// Send sequence of messages rapidly
		numMessages := 10
		for i := 0; i < numMessages; i++ {
			err := tempDataFn(map[string]interface{}{
				"temperature": 20.0 + float64(i),
				"location":    "sequence-test",
				"sequence":    i,
			})
			if err != nil {
				t.Errorf("Failed to publish sequence message %d: %v", i, err)
			}
			time.Sleep(10 * time.Millisecond) // Rapid sequence
		}

		// Wait for all messages
		time.Sleep(1 * time.Second)

		tempMu.Lock()
		defer tempMu.Unlock()

		if len(tempMessages) != numMessages {
			t.Errorf("Expected %d messages, got %d", numMessages, len(tempMessages))
		}

		// Verify message ordering (if sequence field is preserved)
		for i, msg := range tempMessages {
			if sequence, ok := msg["sequence"]; ok {
				if seqNum, ok := sequence.(float64); ok && int(seqNum) != i {
					t.Errorf("Message order violation: expected sequence %d, got %f at position %d", i, seqNum, i)
				}
			}
		}
	})
}

// Test cross-device communication via pub/sub
func TestCrossDeviceCommunication(t *testing.T) {
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

	// Create motion sensor
	motionSensor := newQuietClient("motion-sensor", client.NewTCPTransport())
	err := motionSensor.AddFeature(proto.Feature{
		Name: "motion-detection",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"motion_detected": {Type: "bool"},
					"location":        {Type: "string"},
					"confidence":      {Type: "number", Range: []float64{0, 100}},
					"timestamp":       {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add motion sensor feature: %v", err)
	}

	// Create smart light that reacts to motion
	smartLight := newQuietClient("smart-light", client.NewTCPTransport())

	var lightState bool
	var lightBrightness int
	var lightMu sync.Mutex

	err = smartLight.AddFeature(proto.Feature{
		Name: "light-control",
		Methods: proto.FeatureMethods{
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"on":         {Type: "bool"},
					"brightness": {Type: "number", Range: []float64{0, 100}},
					"mode":       {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add smart light feature: %v", err)
	}

	// Smart light subscribes to motion detection
	err = smartLight.Subscribe("motion-detection", func(msg proto.Message) error {
		lightMu.Lock()
		defer lightMu.Unlock()

		var motionData map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &motionData); err != nil {
			return err
		}

		if motionDetected, ok := motionData["motion_detected"].(bool); ok {
			if motionDetected {
				// Turn on light with high brightness
				lightState = true
				lightBrightness = 90
			} else {
				// Dim the light
				lightState = true
				lightBrightness = 20
			}

			// Publish light status update
			statusFn, err := smartLight.GetStatusFunction("light-control")
			if err != nil {
				return err
			}

			return statusFn(map[string]interface{}{
				"on":         lightState,
				"brightness": lightBrightness,
				"mode":       "motion-activated",
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe light to motion: %v", err)
	}

	// Create controller to monitor light status
	controller := newQuietClient("automation-controller", client.NewTCPTransport())

	var lightStatusUpdates []map[string]interface{}
	var statusMu sync.Mutex

	err = controller.Subscribe("light-control", func(msg proto.Message) error {
		statusMu.Lock()
		defer statusMu.Unlock()

		var status map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &status); err != nil {
			return err
		}
		lightStatusUpdates = append(lightStatusUpdates, status)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe controller to light status: %v", err)
	}

	// Start all devices
	devices := []*client.Client{motionSensor, smartLight, controller}
	for i, device := range devices {
		go func(idx int, dev *client.Client) {
			if err := dev.Start(serverAddr); err != nil {
				t.Errorf("Device %d failed: %v", idx, err)
			}
		}(i, device)
	}

	// Wait for connections
	time.Sleep(1 * time.Second)

	// Test device coordination workflow
	t.Run("MotionActivatedLighting", func(t *testing.T) {
		statusMu.Lock()
		lightStatusUpdates = []map[string]interface{}{}
		statusMu.Unlock()

		// Get motion sensor data function
		motionDataFn, err := motionSensor.GetDataFunction("motion-detection")
		if err != nil {
			t.Fatalf("Failed to get motion data function: %v", err)
		}

		// Simulate motion detection
		err = motionDataFn(map[string]interface{}{
			"motion_detected": true,
			"location":        "living-room",
			"confidence":      95.0,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish motion detection: %v", err)
		}

		// Wait for automation chain to complete
		time.Sleep(1 * time.Second)

		// Verify light responded to motion
		statusMu.Lock()
		defer statusMu.Unlock()

		if len(lightStatusUpdates) == 0 {
			t.Error("Light did not respond to motion detection")
		} else {
			lastStatus := lightStatusUpdates[len(lightStatusUpdates)-1]

			if on, ok := lastStatus["on"].(bool); !ok || !on {
				t.Error("Light should be on after motion detection")
			}

			if brightness, ok := lastStatus["brightness"].(float64); !ok || brightness != 90 {
				t.Errorf("Expected brightness 90 after motion, got %v", lastStatus["brightness"])
			}

			if mode, ok := lastStatus["mode"].(string); !ok || mode != "motion-activated" {
				t.Errorf("Expected mode 'motion-activated', got %v", lastStatus["mode"])
			}
		}
	})

	// Test no motion scenario
	t.Run("NoMotionScenario", func(t *testing.T) {
		statusMu.Lock()
		lightStatusUpdates = []map[string]interface{}{}
		statusMu.Unlock()

		motionDataFn, err := motionSensor.GetDataFunction("motion-detection")
		if err != nil {
			t.Fatalf("Failed to get motion data function: %v", err)
		}

		// Simulate no motion detected
		err = motionDataFn(map[string]interface{}{
			"motion_detected": false,
			"location":        "living-room",
			"confidence":      85.0,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish no motion: %v", err)
		}

		// Wait for automation response
		time.Sleep(1 * time.Second)

		statusMu.Lock()
		defer statusMu.Unlock()

		if len(lightStatusUpdates) > 0 {
			lastStatus := lightStatusUpdates[len(lightStatusUpdates)-1]

			// Light should be dimmed but still on
			if brightness, ok := lastStatus["brightness"].(float64); !ok || brightness != 20 {
				t.Errorf("Expected dimmed brightness 20 after no motion, got %v", lastStatus["brightness"])
			}
		}
	})
}

// Test subscription persistence across reconnection
func TestSubscriptionPersistence(t *testing.T) {
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

	// Create persistent subscriber
	subscriber := newQuietClient("persistent-subscriber", client.NewTCPTransport())

	var messagesReceived []map[string]interface{}
	var msgMu sync.Mutex

	err := subscriber.Subscribe("persistent-topic", func(msg proto.Message) error {
		msgMu.Lock()
		defer msgMu.Unlock()

		var data map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			return err
		}
		messagesReceived = append(messagesReceived, data)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Start subscriber
	go func() {
		// This will handle reconnection automatically
		if err := subscriber.Start(serverAddr); err != nil {
			t.Logf("Subscriber connection error (expected during reconnect): %v", err)
		}
	}()

	// Wait for initial connection
	time.Sleep(500 * time.Millisecond)

	// Verify initial subscription
	subs := broker.Subs("persistent-topic")
	if len(subs) == 0 {
		t.Error("Initial subscription not registered")
	}

	// Create publisher after subscriber is connected
	publisher := newQuietClient("test-publisher", client.NewTCPTransport())
	err = publisher.AddFeature(proto.Feature{
		Name: "persistent-topic",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"test_data": {Type: "string"},
					"sequence":  {Type: "number"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add publisher feature: %v", err)
	}

	go func() {
		if err := publisher.Start(serverAddr); err != nil {
			t.Errorf("Publisher failed: %v", err)
		}
	}()

	// Wait for publisher connection
	time.Sleep(500 * time.Millisecond)

	// Test message delivery before reconnection
	t.Run("PreReconnectionDelivery", func(t *testing.T) {
		msgMu.Lock()
		messagesReceived = []map[string]interface{}{}
		msgMu.Unlock()

		dataFn, err := publisher.GetDataFunction("persistent-topic")
		if err != nil {
			t.Fatalf("Failed to get data function: %v", err)
		}

		err = dataFn(map[string]interface{}{
			"test_data": "pre-reconnect",
			"sequence":  1,
		})
		if err != nil {
			t.Fatalf("Failed to publish pre-reconnect message: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		msgMu.Lock()
		defer msgMu.Unlock()

		if len(messagesReceived) != 1 {
			t.Errorf("Expected 1 pre-reconnect message, got %d", len(messagesReceived))
		}
	})

	// Note: Full reconnection testing would require simulating network failures
	// For this integration test, we verify the subscription framework is in place
	// Real reconnection testing would be done in a more complex test environment
}

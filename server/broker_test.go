package server

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mbocsi/gohab/proto"
)

// MockClient for testing broker functionality
type MockClient struct {
	id       string
	metadata *DeviceMetadata
	messages []proto.Message
	sendErr  error
	mu       sync.Mutex
}

func NewMockClient(id string) *MockClient {
	return &MockClient{
		id:       id,
		metadata: &DeviceMetadata{Id: id, Features: make(map[string]proto.Feature), Subs: make(map[string]struct{})},
		messages: make([]proto.Message, 0),
	}
}

func (mc *MockClient) Send(msg proto.Message) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.sendErr != nil {
		return mc.sendErr
	}
	mc.messages = append(mc.messages, msg)
	return nil
}

func (mc *MockClient) Meta() *DeviceMetadata {
	return mc.metadata
}

func (mc *MockClient) GetMessages() []proto.Message {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	result := make([]proto.Message, len(mc.messages))
	copy(result, mc.messages)
	return result
}

func (mc *MockClient) SetSendError(err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.sendErr = err
}

func (mc *MockClient) ClearMessages() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.messages = mc.messages[:0]
}

func TestNewBroker(t *testing.T) {
	broker := NewBroker()
	
	if broker == nil {
		t.Fatal("Expected broker to be created")
	}
	
	if broker.subs == nil {
		t.Error("Expected subscriptions map to be initialized")
	}
}

func TestBroker_Subscribe(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	broker.Subscribe(topic, client)
	
	subs := broker.Subs(topic)
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subs))
	}
	
	if _, exists := subs[client]; !exists {
		t.Error("Expected client to be subscribed to topic")
	}
}

func TestBroker_Subscribe_MultipleSameClient(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	// Subscribe the same client multiple times
	broker.Subscribe(topic, client)
	broker.Subscribe(topic, client)
	
	subs := broker.Subs(topic)
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber after duplicate subscription, got %d", len(subs))
	}
}

func TestBroker_Subscribe_MultipleClients(t *testing.T) {
	broker := NewBroker()
	client1 := NewMockClient("client-1")
	client2 := NewMockClient("client-2")
	client3 := NewMockClient("client-3")
	topic := "test/topic"
	
	broker.Subscribe(topic, client1)
	broker.Subscribe(topic, client2)
	broker.Subscribe(topic, client3)
	
	subs := broker.Subs(topic)
	if len(subs) != 3 {
		t.Errorf("Expected 3 subscribers, got %d", len(subs))
	}
	
	for _, client := range []Client{client1, client2, client3} {
		if _, exists := subs[client]; !exists {
			t.Errorf("Expected client %s to be subscribed", client.Meta().Id)
		}
	}
}

func TestBroker_Subscribe_MultipleTopics(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	
	broker.Subscribe("topic1", client)
	broker.Subscribe("topic2", client)
	broker.Subscribe("topic3", client)
	
	for _, topic := range []string{"topic1", "topic2", "topic3"} {
		subs := broker.Subs(topic)
		if len(subs) != 1 {
			t.Errorf("Expected 1 subscriber for topic %s, got %d", topic, len(subs))
		}
		
		if _, exists := subs[client]; !exists {
			t.Errorf("Expected client to be subscribed to topic %s", topic)
		}
	}
}

func TestBroker_Publish_SingleSubscriber(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	broker.Subscribe(topic, client)
	
	payloadData := map[string]string{"data": "test"}
	payloadBytes, _ := json.Marshal(payloadData)
	msg := proto.Message{
		Type:      "data",
		Topic:     topic,
		Payload:   payloadBytes,
		Sender:    "sender-id",
		Timestamp: time.Now().Unix(),
	}
	
	broker.Publish(msg)
	
	messages := client.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	if messages[0].Type != msg.Type {
		t.Errorf("Expected message type %s, got %s", msg.Type, messages[0].Type)
	}
	
	if messages[0].Topic != msg.Topic {
		t.Errorf("Expected message topic %s, got %s", msg.Topic, messages[0].Topic)
	}
}

func TestBroker_Publish_MultipleSubscribers(t *testing.T) {
	broker := NewBroker()
	client1 := NewMockClient("client-1")
	client2 := NewMockClient("client-2")
	client3 := NewMockClient("client-3")
	topic := "test/topic"
	
	broker.Subscribe(topic, client1)
	broker.Subscribe(topic, client2)
	broker.Subscribe(topic, client3)
	
	payloadData := map[string]string{"data": "broadcast"}
	payloadBytes, _ := json.Marshal(payloadData)
	msg := proto.Message{
		Type:      "data",
		Topic:     topic,
		Payload:   payloadBytes,
		Sender:    "broadcaster",
		Timestamp: time.Now().Unix(),
	}
	
	broker.Publish(msg)
	
	for _, client := range []*MockClient{client1, client2, client3} {
		messages := client.GetMessages()
		if len(messages) != 1 {
			t.Errorf("Expected 1 message for client %s, got %d", client.id, len(messages))
		}
		
		if len(messages) > 0 && messages[0].Topic != topic {
			t.Errorf("Expected message topic %s for client %s, got %s", topic, client.id, messages[0].Topic)
		}
	}
}

func TestBroker_Publish_NoSubscribers(t *testing.T) {
	broker := NewBroker()
	
	msg := proto.Message{
		Type:    "data",
		Topic:   "nonexistent/topic",
		Payload: []byte(`{"data": "test"}`),
		Sender:  "sender-id",
	}
	
	// Should not panic when publishing to topic with no subscribers
	broker.Publish(msg)
}

func TestBroker_Publish_WithSendError(t *testing.T) {
	broker := NewBroker()
	client1 := NewMockClient("client-1")
	client2 := NewMockClient("client-2")
	topic := "test/topic"
	
	broker.Subscribe(topic, client1)
	broker.Subscribe(topic, client2)
	
	// Make client1 return an error when sending
	client1.SetSendError(errors.New("mock send error"))
	
	msg := proto.Message{
		Type:    "data",
		Topic:   topic,
		Payload: []byte(`{"data": "test"}`),
		Sender:  "sender-id",
	}
	
	// Should not panic and should continue to send to other clients
	broker.Publish(msg)
	
	// client1 should have no messages due to error
	messages1 := client1.GetMessages()
	if len(messages1) != 0 {
		t.Errorf("Expected 0 messages for client1 due to error, got %d", len(messages1))
	}
	
	// client2 should receive the message
	messages2 := client2.GetMessages()
	if len(messages2) != 1 {
		t.Errorf("Expected 1 message for client2, got %d", len(messages2))
	}
}

func TestBroker_Unsubscribe(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	// Subscribe then unsubscribe
	broker.Subscribe(topic, client)
	
	subs := broker.Subs(topic)
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber before unsubscribe, got %d", len(subs))
	}
	
	broker.Unsubscribe(topic, client)
	
	subs = broker.Subs(topic)
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", len(subs))
	}
}

func TestBroker_Unsubscribe_NotSubscribed(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	// Unsubscribe from topic client was never subscribed to
	broker.Unsubscribe(topic, client)
	
	// Should not panic and should have no effect
	subs := broker.Subs(topic)
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers, got %d", len(subs))
	}
}

func TestBroker_Unsubscribe_OneOfMultiple(t *testing.T) {
	broker := NewBroker()
	client1 := NewMockClient("client-1")
	client2 := NewMockClient("client-2")
	client3 := NewMockClient("client-3")
	topic := "test/topic"
	
	// Subscribe all clients
	broker.Subscribe(topic, client1)
	broker.Subscribe(topic, client2)
	broker.Subscribe(topic, client3)
	
	// Unsubscribe only client2
	broker.Unsubscribe(topic, client2)
	
	subs := broker.Subs(topic)
	if len(subs) != 2 {
		t.Errorf("Expected 2 subscribers after unsubscribe, got %d", len(subs))
	}
	
	if _, exists := subs[client1]; !exists {
		t.Error("Expected client1 to still be subscribed")
	}
	
	if _, exists := subs[client2]; exists {
		t.Error("Expected client2 to be unsubscribed")
	}
	
	if _, exists := subs[client3]; !exists {
		t.Error("Expected client3 to still be subscribed")
	}
}

func TestBroker_Subs_NonexistentTopic(t *testing.T) {
	broker := NewBroker()
	
	subs := broker.Subs("nonexistent/topic")
	if subs == nil {
		t.Error("Expected empty map, got nil")
	}
	
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers for nonexistent topic, got %d", len(subs))
	}
}

func TestBroker_Subs_ReturnsCopy(t *testing.T) {
	broker := NewBroker()
	client := NewMockClient("test-client")
	topic := "test/topic"
	
	broker.Subscribe(topic, client)
	
	subs1 := broker.Subs(topic)
	subs2 := broker.Subs(topic)
	
	// Should return different maps (copies)
	if &subs1 == &subs2 {
		t.Error("Expected different map instances")
	}
	
	// But with same content
	if len(subs1) != len(subs2) {
		t.Error("Expected same content in both copies")
	}
	
	// Modifying one should not affect the other or the original
	delete(subs1, client)
	
	if len(subs2) != 1 {
		t.Error("Modifying copy should not affect other copy")
	}
	
	subs3 := broker.Subs(topic)
	if len(subs3) != 1 {
		t.Error("Modifying copy should not affect original")
	}
}

func TestBroker_ConcurrentAccess(t *testing.T) {
	broker := NewBroker()
	topic := "test/topic"
	numClients := 10
	numMessages := 5
	
	var wg sync.WaitGroup
	
	// Start goroutines to subscribe clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := NewMockClient(string(rune(id)))
			broker.Subscribe(topic, client)
		}(i)
	}
	
	// Start goroutines to publish messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(msgID int) {
			defer wg.Done()
			msg := proto.Message{
				Type:    "data",
				Topic:   topic,
				Payload: []byte(`{"id": "` + string(rune(msgID)) + `"}`),
				Sender:  "sender",
			}
			broker.Publish(msg)
		}(i)
	}
	
	// Start goroutines to get subscribers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			broker.Subs(topic)
		}()
	}
	
	wg.Wait()
	
	// Test should complete without race conditions or panics
	subs := broker.Subs(topic)
	if len(subs) > numClients {
		t.Errorf("Expected at most %d subscribers, got %d", numClients, len(subs))
	}
}
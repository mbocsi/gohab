package server

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mbocsi/gohab/proto"
)

// mockLoRaRadio provides a simple mock for testing LoRaTransport
type mockLoRaRadio struct {
	running     bool
	msgChannel  chan LoRaMessage
	mu          sync.RWMutex
}

func NewMockLoRaRadio() *mockLoRaRadio {
	return &mockLoRaRadio{
		msgChannel: make(chan LoRaMessage, 100),
	}
}

func (r *mockLoRaRadio) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = true
	return nil
}

func (r *mockLoRaRadio) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	close(r.msgChannel)
	return nil
}

func (r *mockLoRaRadio) Send(address []byte, data []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.running {
		return fmt.Errorf("radio not running")
	}
	return nil
}

func (r *mockLoRaRadio) Receive() (LoRaMessage, error) {
	select {
	case msg, ok := <-r.msgChannel:
		if !ok {
			return LoRaMessage{}, fmt.Errorf("radio stopped")
		}
		return msg, nil
	case <-time.After(100 * time.Millisecond):
		return LoRaMessage{}, fmt.Errorf("no message available")
	}
}

func (r *mockLoRaRadio) SimulateMessage(address []byte, data []byte, rssi int, snr float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.running {
		r.msgChannel <- LoRaMessage{
			DeviceAddress: address,
			Data:         data,
			RSSI:         rssi,
			SNR:          snr,
		}
	}
}

func TestNewLoRaTransport(t *testing.T) {
	config := LoRaConfig{
		Frequency:       868000000,
		Bandwidth:       125000,
		SpreadingFactor: 7,
		CodingRate:      5,
		TxPower:         14,
	}

	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)

	if transport == nil {
		t.Fatal("NewLoRaTransport returned nil")
	}

	if transport.config != config {
		t.Error("Config not properly set")
	}

	if transport.maxClients != 50 {
		t.Errorf("Expected maxClients to be 50, got %d", transport.maxClients)
	}

	if transport.clients == nil {
		t.Error("Clients map not initialized")
	}

	if transport.radio == nil {
		t.Error("Mock radio not initialized")
	}
}

func TestLoRaTransportMetadata(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)
	transport.SetName("Test LoRa Gateway")
	transport.SetDescription("Test transport")

	meta := transport.Meta()

	if meta.ID != "lora-868000000" {
		t.Errorf("Expected ID 'lora-868000000', got '%s'", meta.ID)
	}

	if meta.Protocol != "lora" {
		t.Errorf("Expected protocol 'lora', got '%s'", meta.Protocol)
	}

	if meta.Address != "868.0MHz" {
		t.Errorf("Expected address '868.0MHz', got '%s'", meta.Address)
	}

	if meta.Name != "Test LoRa Gateway" {
		t.Errorf("Expected name 'Test LoRa Gateway', got '%s'", meta.Name)
	}

	if meta.Connected != false {
		t.Error("Expected Connected to be false before start")
	}
}

func TestLoRaTransportStartShutdown(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)

	// Test start without callbacks should fail
	err := transport.Start()
	if err == nil {
		t.Error("Expected error when starting without callbacks")
	}

	// Set required callbacks
	transport.OnMessage(func(msg proto.Message) {
		// Message handler for start
	})

	transport.OnConnect(func(client Client) error {
		// Connect handler for start
		return nil
	})

	transport.OnDisconnect(func(client Client) {
		// Disconnect handler for start
	})

	// Now start should succeed
	go func() {
		err = transport.Start()
		if err != nil {
			t.Errorf("Expected start to succeed, got error: %v", err)
		}
	}()
	
	// Wait for transport to start
	time.Sleep(10 * time.Millisecond)

	if !transport.connected {
		t.Error("Expected transport to be connected after start")
	}

	// Test shutdown
	err = transport.Shutdown()
	if err != nil {
		t.Fatalf("Expected shutdown to succeed, got error: %v", err)
	}

	if transport.connected {
		t.Error("Expected transport to be disconnected after shutdown")
	}
}

func TestLoRaMessageHandling(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)

	var receivedMessage proto.Message
	var connectedClient Client

	transport.OnMessage(func(msg proto.Message) {
		receivedMessage = msg
	})

	transport.OnConnect(func(client Client) error {
		connectedClient = client
		return nil
	})

	transport.OnDisconnect(func(client Client) {
		// Not tested in this case
	})

	go func() {
		err := transport.Start()
		if err != nil {
			t.Errorf("Failed to start transport: %v", err)
		}
	}()
	defer transport.Shutdown()
	
	// Wait for transport to start
	time.Sleep(10 * time.Millisecond)

	// Simulate a device sending an identify message
	deviceAddr := []byte{0x01, 0x02, 0x03, 0x04}
	identifyMsg := proto.Message{
		Type:      "identify",
		Payload:   json.RawMessage(`{"name":"test-device"}`),
		Timestamp: time.Now().Unix(),
	}

	msgData, _ := json.Marshal(identifyMsg)
	radio.SimulateMessage(deviceAddr, msgData, -80, 5.5)

	// Give some time for message processing
	time.Sleep(10 * time.Millisecond)

	if connectedClient == nil {
		t.Error("Expected client to be connected")
	}

	if receivedMessage.Type == "" {
		t.Error("Expected to receive message")
	}

	if receivedMessage.Type != "identify" {
		t.Errorf("Expected message type 'identify', got '%s'", receivedMessage.Type)
	}

	if receivedMessage.Sender == "" {
		t.Error("Expected Sender to be set")
	}

	// Check that client is in transport's client map
	meta := transport.Meta()
	if len(meta.Clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(meta.Clients))
	}
}

func TestNewLoRaClient(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)
	address := []byte{0x01, 0x02, 0x03, 0x04}
	rssi := -75
	snr := 8.5

	client := NewLoRaClient(address, rssi, snr, transport)

	if client == nil {
		t.Fatal("NewLoRaClient returned nil")
	}

	meta := client.Meta()
	if meta.Id == "" {
		t.Error("Client ID not set")
	}

	if len(meta.Id) < 5 || meta.Id[:5] != "lora-" {
		t.Errorf("Expected ID to start with 'lora-', got '%s'", meta.Id)
	}

	if client.Transport != transport {
		t.Error("Transport reference not set correctly")
	}

	clientAddr := client.GetAddress()
	if len(clientAddr) != len(address) {
		t.Error("Address not set correctly")
	}

	for i := range address {
		if clientAddr[i] != address[i] {
			t.Error("Address bytes don't match")
			break
		}
	}

	clientRSSI, clientSNR := client.GetSignalQuality()
	if clientRSSI != rssi {
		t.Errorf("Expected RSSI %d, got %d", rssi, clientRSSI)
	}

	if clientSNR != snr {
		t.Errorf("Expected SNR %f, got %f", snr, clientSNR)
	}
}

func TestLoRaClientSend(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)
	address := []byte{0x01, 0x02, 0x03, 0x04}
	
	err := transport.Start()
	if err != nil {
		// Set minimal callbacks for start to work
		transport.OnMessage(func(msg proto.Message) {})
		transport.OnConnect(func(client Client) error { return nil })
		transport.OnDisconnect(func(client Client) {})
		go func() {
			err = transport.Start()
			if err != nil {
				t.Errorf("Failed to start transport: %v", err)
			}
		}()
		// Wait for transport to start
		time.Sleep(10 * time.Millisecond)
	}
	defer transport.Shutdown()

	client := NewLoRaClient(address, -80, 5.5, transport)

	testMsg := proto.Message{
		Type:      "data",
		Topic:     "temperature",
		Payload:   json.RawMessage(`{"value":22.5}`),
		Timestamp: time.Now().Unix(),
	}

	err = client.Send(testMsg)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}
}

func TestLoRaClientSignalQualityUpdate(t *testing.T) {
	config := LoRaConfig{Frequency: 868000000}
	radio := NewMockLoRaRadio()
	transport := NewLoRaTransport(config, radio)
	address := []byte{0x01, 0x02, 0x03, 0x04}

	client := NewLoRaClient(address, -80, 5.5, transport)

	// Update signal quality
	newRSSI := -70
	newSNR := 7.2
	client.updateSignalQuality(newRSSI, newSNR)

	rssi, snr := client.GetSignalQuality()
	if rssi != newRSSI {
		t.Errorf("Expected updated RSSI %d, got %d", newRSSI, rssi)
	}

	if snr != newSNR {
		t.Errorf("Expected updated SNR %f, got %f", newSNR, snr)
	}
}

func TestMockLoRaRadio(t *testing.T) {
	radio := NewMockLoRaRadio()

	err := radio.Start()
	if err != nil {
		t.Errorf("Failed to start radio: %v", err)
	}

	if !radio.running {
		t.Error("Expected radio to be running")
	}

	// Test send
	testData := []byte("test message")
	testAddr := []byte{0x01, 0x02}
	err = radio.Send(testAddr, testData)
	if err != nil {
		t.Errorf("Failed to send: %v", err)
	}

	// Test simulate and receive
	radio.SimulateMessage(testAddr, testData, -75, 6.0)
	
	msg, err := radio.Receive()
	if err != nil {
		t.Errorf("Failed to receive: %v", err)
	}

	if len(msg.DeviceAddress) != len(testAddr) {
		t.Error("Device address length mismatch")
	}

	if len(msg.Data) != len(testData) {
		t.Error("Data length mismatch")
	}

	if msg.RSSI != -75 {
		t.Errorf("Expected RSSI -75, got %d", msg.RSSI)
	}

	if msg.SNR != 6.0 {
		t.Errorf("Expected SNR 6.0, got %f", msg.SNR)
	}

	err = radio.Stop()
	if err != nil {
		t.Errorf("Failed to stop radio: %v", err)
	}

	if radio.running {
		t.Error("Expected radio to be stopped")
	}
}
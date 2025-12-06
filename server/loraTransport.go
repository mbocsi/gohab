package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mbocsi/gohab/proto"
)

// LoRaConfig contains basic LoRa radio configuration
type LoRaConfig struct {
	Frequency       uint32 // Hz (e.g., 868000000 for 868MHz)
	Bandwidth       uint32 // Hz (e.g., 125000 for 125kHz)
	SpreadingFactor uint8  // 7-12
	CodingRate      uint8  // 5-8
	TxPower         uint8  // dBm
}

// LoRaMessage represents a received LoRa message with metadata
type LoRaMessage struct {
	DeviceAddress []byte
	Data         []byte
	RSSI         int
	SNR          float64
}

// LoRaRadio defines the interface for LoRa radio hardware
type LoRaRadio interface {
	Start() error
	Stop() error
	Send(address []byte, data []byte) error
	Receive() (LoRaMessage, error)
}

// LoRaTransport implements Transport interface for LoRa communication
type LoRaTransport struct {
	config LoRaConfig
	radio  LoRaRadio

	onMessage    func(proto.Message)
	onConnect    func(Client) error
	onDisconnect func(Client)

	name        string
	description string
	clients     map[string]Client
	cmu         sync.RWMutex

	maxClients int
	connected  bool
	running    bool
}

func NewLoRaTransport(config LoRaConfig, radio LoRaRadio) *LoRaTransport {
	return &LoRaTransport{
		config:     config,
		radio:      radio,
		maxClients: 50, // LoRa can handle many low-bandwidth devices
		clients:    make(map[string]Client),
	}
}

func (t *LoRaTransport) Start() error {
	slog.Info("Starting LoRa transport", "frequency", t.config.Frequency)

	if t.onConnect == nil || t.onDisconnect == nil || t.onMessage == nil {
		return fmt.Errorf("OnConnect, OnDisconnect, or OnMessage function is not defined")
	}

	err := t.radio.Start()
	if err != nil {
		return fmt.Errorf("failed to start LoRa radio: %w", err)
	}

	t.connected = true
	t.running = true

	// Start message processing loop (blocks like other transports)
	t.messageLoop()

	return nil
}

func (t *LoRaTransport) messageLoop() {
	for t.running {
		msg, err := t.radio.Receive()
		if err != nil {
			// No message available, continue
			continue
		}

		t.handleMessage(msg)
	}
}

func (t *LoRaTransport) handleMessage(loraMsg LoRaMessage) {
	deviceAddr := fmt.Sprintf("%x", loraMsg.DeviceAddress)
	
	// Parse JSON message
	var msg proto.Message
	if err := json.Unmarshal(loraMsg.Data, &msg); err != nil {
		slog.Warn("Invalid JSON message from LoRa device", "error", err, "address", deviceAddr)
		return
	}

	// Check if this is a new device
	t.cmu.RLock()
	client, exists := t.clients[deviceAddr]
	t.cmu.RUnlock()

	if !exists {
		// New device - create client and register
		client = NewLoRaClient(loraMsg.DeviceAddress, loraMsg.RSSI, loraMsg.SNR, t)
		
		err := t.onConnect(client)
		if err != nil {
			slog.Error("Failed to register LoRa device", "address", deviceAddr, "error", err)
			return
		}

		t.cmu.Lock()
		t.clients[client.Meta().Id] = client
		t.cmu.Unlock()

		slog.Info("New LoRa device connected", "address", deviceAddr, "id", client.Meta().Id)
	} else {
		// Update signal quality for existing device
		if loraClient, ok := client.(*LoRaClient); ok {
			loraClient.updateSignalQuality(loraMsg.RSSI, loraMsg.SNR)
		}
	}

	// Inject client ID and forward message to server
	msg.Sender = client.Meta().Id
	slog.Debug("LoRa message received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "rssi", loraMsg.RSSI)
	t.onMessage(msg)
}

func (t *LoRaTransport) Shutdown() error {
	slog.Info("Shutting down LoRa transport")
	t.running = false
	t.connected = false
	
	if t.radio != nil {
		return t.radio.Stop()
	}
	return nil
}

func (t *LoRaTransport) OnMessage(fn func(proto.Message)) {
	t.onMessage = fn
}

func (t *LoRaTransport) OnConnect(fn func(Client) error) {
	t.onConnect = fn
}

func (t *LoRaTransport) OnDisconnect(fn func(Client)) {
	t.onDisconnect = fn
}

func (t *LoRaTransport) Meta() TransportMetadata {
	t.cmu.RLock()
	clients := make(map[string]Client)
	for id, client := range t.clients {
		clients[id] = client
	}
	t.cmu.RUnlock()

	return TransportMetadata{
		ID:          fmt.Sprintf("lora-%d", t.config.Frequency),
		Name:        t.name,
		Description: t.description,
		Protocol:    "lora",
		Address:     fmt.Sprintf("%.1fMHz", float64(t.config.Frequency)/1000000),
		Clients:     clients,
		MaxClients:  t.maxClients,
		Connected:   t.connected,
	}
}

func (t *LoRaTransport) SetName(name string) {
	t.name = name
}

func (t *LoRaTransport) SetMaxClients(n int) {
	t.maxClients = n
}

func (t *LoRaTransport) SetDescription(description string) {
	t.description = description
}

// GetRadio returns the radio interface for testing
func (t *LoRaTransport) GetRadio() LoRaRadio {
	return t.radio
}
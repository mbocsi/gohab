package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mbocsi/gohab/proto"
)

// LoRaClient represents a LoRa device connected through the transport
type LoRaClient struct {
	DeviceMetadata
	address []byte    // LoRa device address
	rssi    int       // Signal strength indicator
	snr     float64   // Signal-to-noise ratio
	mu      sync.RWMutex
}

func NewLoRaClient(address []byte, rssi int, snr float64, transport Transport) *LoRaClient {
	return &LoRaClient{
		address: address,
		rssi:    rssi,
		snr:     snr,
		DeviceMetadata: DeviceMetadata{
			Id:        generateClientId("lora"),
			LastSeen:  time.Now(),
			Features:  make(map[string]proto.Feature),
			Subs:      make(map[string]struct{}),
			Transport: transport,
		},
	}
}

func (c *LoRaClient) Send(msg proto.Message) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Get the transport and send via LoRa radio
	if loraTransport, ok := c.Transport.(*LoRaTransport); ok {
		err = loraTransport.radio.Send(c.address, jsonData)
		if err != nil {
			return fmt.Errorf("failed to send LoRa message: %w", err)
		}
		
		slog.Debug("Sent LoRa message", "to", c.Meta().Id, "type", msg.Type, "topic", msg.Topic, "size", len(jsonData), "rssi", c.rssi)
	} else {
		return fmt.Errorf("invalid transport type for LoRa client")
	}

	// Update last seen timestamp
	c.DeviceMetadata.Mu.Lock()
	c.DeviceMetadata.LastSeen = time.Now()
	c.DeviceMetadata.Mu.Unlock()

	return nil
}

func (c *LoRaClient) Meta() *DeviceMetadata {
	return &c.DeviceMetadata
}

// GetAddress returns the LoRa device address
func (c *LoRaClient) GetAddress() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.address
}

// GetSignalQuality returns current RSSI and SNR values
func (c *LoRaClient) GetSignalQuality() (int, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rssi, c.snr
}

// updateSignalQuality updates the signal quality metrics
func (c *LoRaClient) updateSignalQuality(rssi int, snr float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rssi = rssi
	c.snr = snr
	
	// Update last seen
	c.DeviceMetadata.Mu.Lock()
	c.DeviceMetadata.LastSeen = time.Now()
	c.DeviceMetadata.Mu.Unlock()
}
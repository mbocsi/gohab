package server

import (
	"fmt"
	"log/slog"
	"sync"
)

// SX1276Radio implements LoRaRadio interface for SX1276/SX1278 hardware
// This is a framework implementation that can be extended with actual hardware drivers
type SX1276Radio struct {
	config SX1276Config

	// Control
	running bool
	mu      sync.RWMutex

	// Message queue for our interface
	msgQueue chan LoRaMessage

	// Hardware interface (to be implemented with actual driver)
	hwInterface HardwareInterface
}

// HardwareInterface abstracts the actual hardware communication
// This allows different implementations (SPI, UART, etc.) to be plugged in
type HardwareInterface interface {
	Initialize() error
	Transmit(data []byte) error
	SetReceiveCallback(callback func(data []byte, rssi int, snr float64))
	Close() error
	SetFrequency(freq uint32) error
	SetPower(power uint8) error
}

// SX1276Config contains hardware-specific configuration for SX1276 LoRa radio
type SX1276Config struct {
	// SPI Configuration
	SPIDevice string // e.g., "/dev/spidev0.0"
	SPISpeed  uint32 // SPI clock speed in Hz (e.g., 1000000 for 1MHz)

	// GPIO Pins (GPIO numbers, not pin numbers)
	ResetGPIO int // GPIO number for radio reset (e.g., 4)
	IRQPin    int // GPIO number for radio interrupt (e.g., 17)
	CS0Pin    int // GPIO number for SPI chip select (e.g., 8)

	// Radio Configuration
	Frequency       uint32 // Operating frequency in Hz (e.g., 868000000 for 868MHz)
	Power           uint8  // TX power in dBm (2-20)
	SyncByte        uint8  // LoRa sync byte (default: 0x12)
	Bandwidth       uint32 // Bandwidth in Hz (125000, 250000, 500000)
	SpreadingFactor uint8  // Spreading factor (6-12)
	CodingRate      uint8  // Coding rate (5-8)
}

// NewSX1276Radio creates a new hardware LoRa radio instance
func NewSX1276Radio(config SX1276Config, hwInterface HardwareInterface) (*SX1276Radio, error) {
	radio := &SX1276Radio{
		config:      config,
		running:     false,
		msgQueue:    make(chan LoRaMessage, 100), // Buffer for received messages
		hwInterface: hwInterface,
	}

	return radio, nil
}

// Start initializes the SX1276 hardware and begins radio operations
func (r *SX1276Radio) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("radio already running")
	}

	// Initialize hardware interface
	err := r.hwInterface.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize hardware interface: %w", err)
	}

	// Configure radio parameters
	err = r.hwInterface.SetFrequency(r.config.Frequency)
	if err != nil {
		r.hwInterface.Close()
		return fmt.Errorf("failed to set frequency: %w", err)
	}

	err = r.hwInterface.SetPower(r.config.Power)
	if err != nil {
		r.hwInterface.Close()
		return fmt.Errorf("failed to set power: %w", err)
	}

	// Set up receive callback
	r.hwInterface.SetReceiveCallback(r.onHardwareReceive)

	r.running = true

	slog.Info("SX1276 LoRa radio started",
		"frequency", r.config.Frequency,
		"power", r.config.Power,
		"spi_device", r.config.SPIDevice,
		"reset_gpio", r.config.ResetGPIO)

	return nil
}

// Stop shuts down the SX1276 radio and releases hardware resources
func (r *SX1276Radio) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false
	close(r.msgQueue)

	// Close hardware interface
	if r.hwInterface != nil {
		r.hwInterface.Close()
	}

	slog.Info("SX1276 LoRa radio stopped")
	return nil
}

// Send transmits data to a specific LoRa device address
func (r *SX1276Radio) Send(address []byte, data []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.running {
		return fmt.Errorf("radio not running")
	}

	// Create packet with address header + data
	// Simple addressing: [address_len][address][data]
	packet := make([]byte, 1+len(address)+len(data))
	packet[0] = uint8(len(address))
	copy(packet[1:], address)
	copy(packet[1+len(address):], data)

	// Send via hardware interface
	err := r.hwInterface.Transmit(packet)
	if err != nil {
		return fmt.Errorf("hardware transmit failed: %w", err)
	}

	slog.Debug("SX1276 packet transmitted",
		"address", fmt.Sprintf("%x", address),
		"data_size", len(data),
		"total_size", len(packet))

	return nil
}

// Receive returns the next received LoRa message (blocking)
// Blocks indefinitely until a message arrives or radio is stopped, similar to TCP
func (r *SX1276Radio) Receive() (LoRaMessage, error) {
	msg, ok := <-r.msgQueue
	if !ok {
		return LoRaMessage{}, fmt.Errorf("radio stopped")
	}
	return msg, nil
}

// onHardwareReceive handles callbacks from the hardware interface
func (r *SX1276Radio) onHardwareReceive(data []byte, rssi int, snr float64) {
	// Validate minimum packet size (at least 1 byte for address length)
	if len(data) < 2 {
		slog.Warn("SX1276 received packet too short", "size", len(data))
		return
	}

	// Extract address from packet header
	addrLen := int(data[0])
	if len(data) < 1+addrLen {
		slog.Warn("SX1276 received packet with invalid address length",
			"declared_addr_len", addrLen, "packet_size", len(data))
		return
	}

	address := data[1 : 1+addrLen]
	payload := data[1+addrLen:]

	loraMsg := LoRaMessage{
		DeviceAddress: address,
		Data:          payload,
		RSSI:          rssi,
		SNR:           snr,
	}

	// Queue message for consumption by transport
	select {
	case r.msgQueue <- loraMsg:
		slog.Debug("SX1276 message queued",
			"address", fmt.Sprintf("%x", address),
			"data_size", len(payload),
			"rssi", rssi,
			"snr", snr)
	default:
		slog.Warn("SX1276 message queue full, dropping packet")
	}
}

// Helper functions for creating common configurations

// DefaultSX1276Config returns a standard configuration for 868MHz operation
func DefaultSX1276Config() SX1276Config {
	return SX1276Config{
		SPIDevice:       "/dev/spidev0.0",
		SPISpeed:        1000000,   // 1MHz
		ResetGPIO:       4,         // GPIO 4
		IRQPin:          17,        // GPIO 17
		CS0Pin:          8,         // GPIO 8 (CE0)
		Frequency:       868000000, // 868MHz
		Power:           14,        // 14dBm
		SyncByte:        0x12,
		Bandwidth:       125000, // 125kHz
		SpreadingFactor: 7,
		CodingRate:      5,
	}
}

// US915Config returns configuration for 915MHz (US ISM band)
func US915Config() SX1276Config {
	config := DefaultSX1276Config()
	config.Frequency = 915000000 // 915MHz
	return config
}

// EU433Config returns configuration for 433MHz (EU ISM band)
func EU433Config() SX1276Config {
	config := DefaultSX1276Config()
	config.Frequency = 433000000 // 433MHz
	return config
}

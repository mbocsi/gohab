package server

import (
	"testing"
	"time"
)

func TestNewSX1276Radio(t *testing.T) {
	config := DefaultSX1276Config()
	
	mockHW := NewMockHardwareInterface()
	
	radio, err := NewSX1276Radio(config, mockHW)
	if err != nil {
		t.Fatalf("Failed to create SX1276Radio: %v", err)
	}

	if radio == nil {
		t.Fatal("NewSX1276Radio returned nil")
	}

	if radio.config != config {
		t.Error("Config not properly set")
	}

	if radio.hwInterface != mockHW {
		t.Error("Hardware interface not properly set")
	}
}

// mockHardwareInterface is a simple mock for testing
type mockHardwareInterface struct {
	initialized    bool
	frequency      uint32
	power          uint8
	rxCallback     func(data []byte, rssi int, snr float64)
	transmitCalled bool
}

func (m *mockHardwareInterface) Initialize() error {
	m.initialized = true
	return nil
}

func (m *mockHardwareInterface) Transmit(data []byte) error {
	m.transmitCalled = true
	return nil
}

func (m *mockHardwareInterface) SetReceiveCallback(callback func(data []byte, rssi int, snr float64)) {
	m.rxCallback = callback
}

func (m *mockHardwareInterface) Close() error {
	m.initialized = false
	return nil
}

func (m *mockHardwareInterface) SetFrequency(freq uint32) error {
	m.frequency = freq
	return nil
}

func (m *mockHardwareInterface) SetPower(power uint8) error {
	m.power = power
	return nil
}

func (m *mockHardwareInterface) simulateReceive(data []byte, rssi int, snr float64) {
	if m.rxCallback != nil && m.initialized {
		m.rxCallback(data, rssi, snr)
	}
}

// NewMockHardwareInterface creates a new mock hardware interface for testing
func NewMockHardwareInterface() *mockHardwareInterface {
	return &mockHardwareInterface{}
}

func TestSX1276Radio_StartStop(t *testing.T) {
	config := DefaultSX1276Config()
	mockHW := NewMockHardwareInterface()
	
	radio, err := NewSX1276Radio(config, mockHW)
	if err != nil {
		t.Fatalf("Failed to create SX1276Radio: %v", err)
	}

	// Test start
	err = radio.Start()
	if err != nil {
		t.Fatalf("Failed to start radio: %v", err)
	}

	if !radio.running {
		t.Error("Radio should be running after start")
	}

	// Test stop
	err = radio.Stop()
	if err != nil {
		t.Fatalf("Failed to stop radio: %v", err)
	}

	if radio.running {
		t.Error("Radio should not be running after stop")
	}
}

func TestSX1276Radio_Send(t *testing.T) {
	config := DefaultSX1276Config()
	mockHW := NewMockHardwareInterface()
	
	radio, err := NewSX1276Radio(config, mockHW)
	if err != nil {
		t.Fatalf("Failed to create SX1276Radio: %v", err)
	}

	err = radio.Start()
	if err != nil {
		t.Fatalf("Failed to start radio: %v", err)
	}
	defer radio.Stop()

	// Test send
	address := []byte{0x01, 0x02, 0x03, 0x04}
	data := []byte(`{"temperature": 22.5}`)

	err = radio.Send(address, data)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	if !mockHW.transmitCalled {
		t.Error("Expected Transmit to be called on hardware interface")
	}
}

func TestSX1276Radio_Receive(t *testing.T) {
	config := DefaultSX1276Config()
	mockHW := NewMockHardwareInterface()
	
	radio, err := NewSX1276Radio(config, mockHW)
	if err != nil {
		t.Fatalf("Failed to create SX1276Radio: %v", err)
	}

	err = radio.Start()
	if err != nil {
		t.Fatalf("Failed to start radio: %v", err)
	}
	defer radio.Stop()

	// Simulate receiving a message
	deviceAddr := []byte{0x01, 0x02, 0x03, 0x04}
	testData := []byte(`{"temperature": 22.5}`)
	
	// Create packet with our addressing scheme: [addr_len][address][data]
	packet := make([]byte, 1+len(deviceAddr)+len(testData))
	packet[0] = uint8(len(deviceAddr))
	copy(packet[1:], deviceAddr)
	copy(packet[1+len(deviceAddr):], testData)

	// Simulate hardware receiving the packet
	mockHW.simulateReceive(packet, -75, 6.5)

	// Wait a bit for message to be processed
	time.Sleep(10 * time.Millisecond)

	// Try to receive the message
	msg, err := radio.Receive()
	if err != nil {
		t.Errorf("Failed to receive message: %v", err)
	} else {
		if len(msg.DeviceAddress) != len(deviceAddr) {
			t.Error("Device address length mismatch")
		}

		for i := range deviceAddr {
			if msg.DeviceAddress[i] != deviceAddr[i] {
				t.Error("Device address mismatch")
				break
			}
		}

		if len(msg.Data) != len(testData) {
			t.Error("Data length mismatch")
		}

		if msg.RSSI != -75 {
			t.Errorf("Expected RSSI -75, got %d", msg.RSSI)
		}

		if msg.SNR != 6.5 {
			t.Errorf("Expected SNR 6.5, got %f", msg.SNR)
		}
	}
}

func TestSX1276Radio_ConfigDefaults(t *testing.T) {
	defaultConfig := DefaultSX1276Config()
	
	if defaultConfig.Frequency != 868000000 {
		t.Errorf("Expected frequency 868000000, got %d", defaultConfig.Frequency)
	}

	if defaultConfig.Power != 14 {
		t.Errorf("Expected power 14, got %d", defaultConfig.Power)
	}

	us915Config := US915Config()
	if us915Config.Frequency != 915000000 {
		t.Errorf("Expected US915 frequency 915000000, got %d", us915Config.Frequency)
	}

	eu433Config := EU433Config()
	if eu433Config.Frequency != 433000000 {
		t.Errorf("Expected EU433 frequency 433000000, got %d", eu433Config.Frequency)
	}
}

func TestMockHardwareInterface(t *testing.T) {
	mockHW := NewMockHardwareInterface()

	// Test initialization
	err := mockHW.Initialize()
	if err != nil {
		t.Errorf("Failed to initialize mock hardware: %v", err)
	}

	// Test frequency setting
	err = mockHW.SetFrequency(868000000)
	if err != nil {
		t.Errorf("Failed to set frequency: %v", err)
	}

	// Test power setting
	err = mockHW.SetPower(14)
	if err != nil {
		t.Errorf("Failed to set power: %v", err)
	}

	// Test transmit
	testData := []byte("test message")
	err = mockHW.Transmit(testData)
	if err != nil {
		t.Errorf("Failed to transmit: %v", err)
	}

	if !mockHW.transmitCalled {
		t.Error("Expected transmitCalled to be true")
	}

	// Test receive callback
	var receivedData []byte
	var receivedRSSI int
	var receivedSNR float64

	mockHW.SetReceiveCallback(func(data []byte, rssi int, snr float64) {
		receivedData = data
		receivedRSSI = rssi
		receivedSNR = snr
	})

	// Simulate receive
	mockHW.simulateReceive(testData, -80, 5.5)

	if len(receivedData) != len(testData) {
		t.Error("Received data length mismatch")
	}

	if receivedRSSI != -80 {
		t.Errorf("Expected RSSI -80, got %d", receivedRSSI)
	}

	if receivedSNR != 5.5 {
		t.Errorf("Expected SNR 5.5, got %f", receivedSNR)
	}

	// Test close
	err = mockHW.Close()
	if err != nil {
		t.Errorf("Failed to close mock hardware: %v", err)
	}
}
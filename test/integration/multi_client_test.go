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

// StatefulLEDClient represents a LED device that maintains brightness state
type StatefulLEDClient struct {
	client       *client.Client
	brightness   int
	isOn         bool
	mu           sync.RWMutex
	stateChanged chan struct{}
	featureName  string
}

func NewStatefulLEDClient(name string, featureName string) *StatefulLEDClient {
	c := &StatefulLEDClient{
		client:       client.NewClient(name, client.NewTCPTransport()),
		featureName:  featureName,
		brightness:   0,
		isOn:         false,
		stateChanged: make(chan struct{}, 10),
	}

	// Add LED feature with command and query support
	ledFeature := proto.Feature{
		Name: featureName,
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"on":         {Type: "bool"},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"on":         {Type: "bool"},
				},
			},
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"on":         {Type: "bool"},
				},
			},
		},
	}

	c.client.AddFeature(ledFeature)

	// Register command handler for brightness and on/off control
	c.client.RegisterCommandHandler(featureName, c.handleCommand)

	// Register query handler to return current state
	c.client.RegisterQueryHandler(featureName, c.handleQuery)

	return c
}

func (l *StatefulLEDClient) handleCommand(msg proto.Message) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	l.mu.Lock()
	stateChanged := false

	if brightness, ok := payload["brightness"]; ok {
		if brightnessFloat, ok := brightness.(float64); ok {
			newBrightness := int(brightnessFloat)
			if newBrightness != l.brightness {
				l.brightness = newBrightness
				stateChanged = true
			}
		}
	}

	if on, ok := payload["on"]; ok {
		if onBool, ok := on.(bool); ok {
			if onBool != l.isOn {
				l.isOn = onBool
				stateChanged = true
			}
		}
	}
	l.mu.Unlock()

	if stateChanged {
		// Send status update
		l.publishStatus()
		select {
		case l.stateChanged <- struct{}{}:
		default:
		}
	}

	return nil
}

func (l *StatefulLEDClient) handleQuery(msg proto.Message) (any, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]interface{}{
		"brightness": l.brightness,
		"on":         l.isOn,
	}, nil
}

func (l *StatefulLEDClient) publishStatus() error {
	statusFn, err := l.client.GetStatusFunction(l.featureName)
	if err != nil {
		return err
	}

	l.mu.RLock()
	state := map[string]interface{}{
		"brightness": l.brightness,
		"on":         l.isOn,
	}
	l.mu.RUnlock()

	return statusFn(state)
}

func (l *StatefulLEDClient) GetState() (int, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.brightness, l.isOn
}

func (l *StatefulLEDClient) Start(serverAddr string) error {
	return l.client.Start(serverAddr)
}

func (l *StatefulLEDClient) WaitForStateChange(timeout time.Duration) bool {
	select {
	case <-l.stateChanged:
		return true
	case <-time.After(timeout):
		return false
	}
}

// StatefulTemperatureSensor represents a temperature sensor that can simulate readings
type StatefulTemperatureSensor struct {
	client      *client.Client
	temperature float64
	mu          sync.RWMutex
}

func NewStatefulTemperatureSensor(name string) *StatefulTemperatureSensor {
	s := &StatefulTemperatureSensor{
		client:      client.NewClient(name, client.NewTCPTransport()),
		temperature: 20.0, // Default room temperature
	}

	// Add temperature sensor feature
	tempFeature := proto.Feature{
		Name: "temperature",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number"},
					"unit":        {Type: "string"},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"temperature": {Type: "number"},
					"unit":        {Type: "string"},
				},
			},
		},
	}

	s.client.AddFeature(tempFeature)
	s.client.RegisterQueryHandler("temperature", s.handleQuery)

	return s
}

func (s *StatefulTemperatureSensor) handleQuery(msg proto.Message) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"temperature": s.temperature,
		"unit":        "celsius",
	}, nil
}

func (s *StatefulTemperatureSensor) SetTemperature(temp float64) {
	s.mu.Lock()
	s.temperature = temp
	s.mu.Unlock()
}

func (s *StatefulTemperatureSensor) PublishReading() error {
	dataFn, err := s.client.GetDataFunction("temperature")
	if err != nil {
		return err
	}

	s.mu.RLock()
	reading := map[string]interface{}{
		"temperature": s.temperature,
		"unit":        "celsius",
	}
	s.mu.RUnlock()

	return dataFn(reading)
}

func (s *StatefulTemperatureSensor) GetTemperature() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.temperature
}

func (s *StatefulTemperatureSensor) Start(serverAddr string) error {
	return s.client.Start(serverAddr)
}

// StatefulThermostat represents a thermostat that can control temperature and respond to commands
type StatefulThermostat struct {
	client       *client.Client
	targetTemp   float64
	currentTemp  float64
	mode         string // "heating", "cooling", "off"
	mu           sync.RWMutex
	stateChanged chan struct{}
}

func NewStatefulThermostat(name string) *StatefulThermostat {
	t := &StatefulThermostat{
		client:       client.NewClient(name, client.NewTCPTransport()),
		targetTemp:   22.0,
		currentTemp:  20.0,
		mode:         "off",
		stateChanged: make(chan struct{}, 10),
	}

	// Add thermostat feature
	thermostatFeature := proto.Feature{
		Name: "thermostat",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"target_temp": {Type: "number"},
					"mode":        {Type: "string"},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"target_temp":  {Type: "number"},
					"current_temp": {Type: "number"},
					"mode":         {Type: "string"},
				},
			},
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"target_temp":  {Type: "number"},
					"current_temp": {Type: "number"},
					"mode":         {Type: "string"},
				},
			},
		},
	}

	t.client.AddFeature(thermostatFeature)
	t.client.RegisterCommandHandler("thermostat", t.handleCommand)
	t.client.RegisterQueryHandler("thermostat", t.handleQuery)

	return t
}

func (t *StatefulThermostat) handleCommand(msg proto.Message) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	t.mu.Lock()
	stateChanged := false

	if targetTemp, ok := payload["target_temp"]; ok {
		if tempFloat, ok := targetTemp.(float64); ok && tempFloat != t.targetTemp {
			t.targetTemp = tempFloat
			stateChanged = true
		}
	}

	if mode, ok := payload["mode"]; ok {
		if modeStr, ok := mode.(string); ok && modeStr != t.mode {
			t.mode = modeStr
			stateChanged = true
		}
	}
	t.mu.Unlock()

	if stateChanged {
		t.publishStatus()
		select {
		case t.stateChanged <- struct{}{}:
		default:
		}
	}

	return nil
}

func (t *StatefulThermostat) handleQuery(msg proto.Message) (any, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"target_temp":  t.targetTemp,
		"current_temp": t.currentTemp,
		"mode":         t.mode,
	}, nil
}

func (t *StatefulThermostat) publishStatus() error {
	statusFn, err := t.client.GetStatusFunction("thermostat")
	if err != nil {
		return err
	}

	t.mu.RLock()
	state := map[string]interface{}{
		"target_temp":  t.targetTemp,
		"current_temp": t.currentTemp,
		"mode":         t.mode,
	}
	t.mu.RUnlock()

	return statusFn(state)
}

func (t *StatefulThermostat) GetState() (float64, float64, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.targetTemp, t.currentTemp, t.mode
}

func (t *StatefulThermostat) SetCurrentTemperature(temp float64) {
	t.mu.Lock()
	t.currentTemp = temp
	t.mu.Unlock()
	t.publishStatus()
}

func (t *StatefulThermostat) Start(serverAddr string) error {
	return t.client.Start(serverAddr)
}

func (t *StatefulThermostat) WaitForStateChange(timeout time.Duration) bool {
	select {
	case <-t.stateChanged:
		return true
	case <-time.After(timeout):
		return false
	}
}

// TestEndToEndLEDControl tests complete LED brightness control workflow
func TestEndToEndLEDControl(t *testing.T) {
	// Start server
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

	// Create LED device
	ledDevice := NewStatefulLEDClient("living-room-led", "living-room-led")

	// Create controller client to send commands
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

	// Wait for clients to connect
	time.Sleep(500 * time.Millisecond)

	// Test 1: Turn on LED and set brightness
	t.Run("TurnOnAndSetBrightness", func(t *testing.T) {
		// Verify initial state
		brightness, isOn := ledDevice.GetState()
		if brightness != 0 || isOn != false {
			t.Errorf("Expected initial state: brightness=0, on=false, got brightness=%d, on=%t", brightness, isOn)
		}

		// Send command to turn on and set brightness to 75
		err := controller.SendCommand("living-room-led", map[string]interface{}{
			"brightness": 75,
			"on":         true,
		})
		if err != nil {
			t.Fatalf("Failed to send command: %v", err)
		}

		// Wait for state change
		if !ledDevice.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		// Verify new state
		brightness, isOn = ledDevice.GetState()
		if brightness != 75 || isOn != true {
			t.Errorf("Expected state: brightness=75, on=true, got brightness=%d, on=%t", brightness, isOn)
		}
	})

	// Test 2: Query LED state
	t.Run("QueryState", func(t *testing.T) {
		response, err := controller.SendQuery("living-room-led", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query LED: %v", err)
		}

		var state map[string]interface{}
		if err := json.Unmarshal(response.Payload, &state); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		brightness, ok := state["brightness"]
		if !ok {
			t.Error("Brightness not found in response")
		} else if int(brightness.(float64)) != 75 {
			t.Errorf("Expected brightness 75, got %v", brightness)
		}

		isOn, ok := state["on"]
		if !ok {
			t.Error("On state not found in response")
		} else if isOn.(bool) != true {
			t.Errorf("Expected on=true, got %v", isOn)
		}
	})

	// Test 3: Dim LED
	t.Run("DimLED", func(t *testing.T) {
		err := controller.SendCommand("living-room-led", map[string]interface{}{
			"brightness": 25,
		})
		if err != nil {
			t.Fatalf("Failed to send dim command: %v", err)
		}

		if !ledDevice.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		brightness, isOn := ledDevice.GetState()
		if brightness != 25 || isOn != true {
			t.Errorf("Expected state: brightness=25, on=true, got brightness=%d, on=%t", brightness, isOn)
		}
	})

	// Test 4: Turn off LED
	t.Run("TurnOffLED", func(t *testing.T) {
		err := controller.SendCommand("living-room-led", map[string]interface{}{
			"on": false,
		})
		if err != nil {
			t.Fatalf("Failed to send turn off command: %v", err)
		}

		if !ledDevice.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		brightness, isOn := ledDevice.GetState()
		if brightness != 25 || isOn != false { // brightness should remain, only on/off changes
			t.Errorf("Expected state: brightness=25, on=false, got brightness=%d, on=%t", brightness, isOn)
		}
	})
}

// TestEndToEndThermostatControl tests complete thermostat control workflow with temperature feedback
func TestEndToEndThermostatControl(t *testing.T) {
	// Start server
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

	// Create thermostat device
	thermostat := NewStatefulThermostat("main-thermostat")

	// Create temperature sensor
	tempSensor := NewStatefulTemperatureSensor("room-temp-sensor")

	// Create controller client
	controller := client.NewClient("home-controller", client.NewTCPTransport())

	// Start all clients
	go func() {
		if err := thermostat.Start(serverAddr); err != nil {
			t.Errorf("Thermostat failed: %v", err)
		}
	}()

	go func() {
		if err := tempSensor.Start(serverAddr); err != nil {
			t.Errorf("Temperature sensor failed: %v", err)
		}
	}()

	go func() {
		if err := controller.Start(serverAddr); err != nil {
			t.Errorf("Controller failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(500 * time.Millisecond)

	// Test 1: Set thermostat to heating mode
	t.Run("SetHeatingMode", func(t *testing.T) {
		// Verify initial state
		targetTemp, currentTemp, mode := thermostat.GetState()
		if targetTemp != 22.0 || currentTemp != 20.0 || mode != "off" {
			t.Errorf("Expected initial state: target=22.0, current=20.0, mode=off, got target=%f, current=%f, mode=%s", targetTemp, currentTemp, mode)
		}

		// Set thermostat to heating mode with target temperature
		err := controller.SendCommand("thermostat", map[string]interface{}{
			"target_temp": 24.0,
			"mode":        "heating",
		})
		if err != nil {
			t.Fatalf("Failed to send thermostat command: %v", err)
		}

		// Wait for state change
		if !thermostat.WaitForStateChange(2 * time.Second) {
			t.Fatal("Thermostat state did not change within timeout")
		}

		// Verify new state
		targetTemp, _, mode = thermostat.GetState()
		if targetTemp != 24.0 || mode != "heating" {
			t.Errorf("Expected state: target=24.0, mode=heating, got target=%f, mode=%s", targetTemp, mode)
		}
	})

	// Test 2: Query thermostat state
	t.Run("QueryThermostatState", func(t *testing.T) {
		response, err := controller.SendQuery("thermostat", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query thermostat: %v", err)
		}

		var state map[string]interface{}
		if err := json.Unmarshal(response.Payload, &state); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		targetTemp := state["target_temp"].(float64)
		mode := state["mode"].(string)

		if targetTemp != 24.0 {
			t.Errorf("Expected target_temp 24.0, got %f", targetTemp)
		}
		if mode != "heating" {
			t.Errorf("Expected mode 'heating', got '%s'", mode)
		}
	})

	// Test 3: Simulate temperature change and verify thermostat response
	t.Run("TemperatureChangeResponse", func(t *testing.T) {
		// Simulate temperature rising due to heating
		tempSensor.SetTemperature(23.5)
		thermostat.SetCurrentTemperature(23.5)

		// Verify thermostat updated its current temperature
		_, currentTemp, _ := thermostat.GetState()
		if currentTemp != 23.5 {
			t.Errorf("Expected current temperature 23.5, got %f", currentTemp)
		}

		// Temperature sensor should report new reading
		sensorTemp := tempSensor.GetTemperature()
		if sensorTemp != 23.5 {
			t.Errorf("Expected sensor temperature 23.5, got %f", sensorTemp)
		}

		// Publish temperature data
		err := tempSensor.PublishReading()
		if err != nil {
			t.Fatalf("Failed to publish temperature reading: %v", err)
		}
	})

	// Test 4: Query temperature sensor
	t.Run("QueryTemperatureSensor", func(t *testing.T) {
		response, err := controller.SendQuery("temperature", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query temperature sensor: %v", err)
		}

		var reading map[string]interface{}
		if err := json.Unmarshal(response.Payload, &reading); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		temp := reading["temperature"].(float64)
		unit := reading["unit"].(string)

		if temp != 23.5 {
			t.Errorf("Expected temperature 23.5, got %f", temp)
		}
		if unit != "celsius" {
			t.Errorf("Expected unit 'celsius', got '%s'", unit)
		}
	})

	// Test 5: Switch to cooling mode when target reached
	t.Run("SwitchToCoolingMode", func(t *testing.T) {
		// Temperature reached target, now switch to cooling
		err := controller.SendCommand("thermostat", map[string]interface{}{
			"target_temp": 22.0,
			"mode":        "cooling",
		})
		if err != nil {
			t.Fatalf("Failed to send cooling command: %v", err)
		}

		if !thermostat.WaitForStateChange(2 * time.Second) {
			t.Fatal("Thermostat state did not change within timeout")
		}

		targetTemp, _, mode := thermostat.GetState()
		if targetTemp != 22.0 || mode != "cooling" {
			t.Errorf("Expected state: target=22.0, mode=cooling, got target=%f, mode=%s", targetTemp, mode)
		}
	})
}

// TestEndToEndPubSubWorkflow tests pub/sub communication with stateful devices
func TestEndToEndPubSubWorkflow(t *testing.T) {
	// Start server
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

	// Create temperature sensor as publisher
	tempSensor := NewStatefulTemperatureSensor("outdoor-sensor")

	// Create LED device that subscribes to temperature changes
	ledDevice := NewStatefulLEDClient("temp-indicator-led", "temp-indicator-led")

	// Track received temperature messages
	var receivedTemperatures []float64
	var temperatureMutex sync.Mutex

	// Set up LED to subscribe to temperature updates
	err := ledDevice.client.Subscribe("temperature", func(msg proto.Message) error {
		var tempData map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &tempData); err != nil {
			return err
		}

		if temp, ok := tempData["temperature"].(float64); ok {
			temperatureMutex.Lock()
			receivedTemperatures = append(receivedTemperatures, temp)
			temperatureMutex.Unlock()

			// Adjust LED brightness based on temperature
			// Temperature range 0-30°C maps to brightness 0-100
			brightness := int((temp / 30.0) * 100)
			if brightness > 100 {
				brightness = 100
			}
			if brightness < 0 {
				brightness = 0
			}

			// Update LED based on temperature
			payload, _ := json.Marshal(map[string]interface{}{
				"brightness": brightness,
				"on":         temp > 0, // Turn on if above freezing
			})
			return ledDevice.handleCommand(proto.Message{
				Payload: payload,
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to set up temperature subscription: %v", err)
	}

	// Start both devices
	go func() {
		if err := tempSensor.Start(serverAddr); err != nil {
			t.Errorf("Temperature sensor failed: %v", err)
		}
	}()

	go func() {
		if err := ledDevice.Start(serverAddr); err != nil {
			t.Errorf("LED device failed: %v", err)
		}
	}()

	// Wait for connections
	time.Sleep(500 * time.Millisecond)

	// Test temperature-LED coordination
	testTemperatures := []float64{15.0, 25.0, 30.0, 10.0}

	for i, temp := range testTemperatures {
		t.Run(fmt.Sprintf("TemperatureUpdate_%d", i+1), func(t *testing.T) {
			// Update temperature
			tempSensor.SetTemperature(temp)

			// Publish temperature reading
			err := tempSensor.PublishReading()
			if err != nil {
				t.Fatalf("Failed to publish temperature: %v", err)
			}

			// Wait for LED to receive update and change state
			if !ledDevice.WaitForStateChange(2 * time.Second) {
				t.Fatal("LED did not respond to temperature change")
			}

			// Verify LED state matches temperature
			brightness, isOn := ledDevice.GetState()
			expectedBrightness := int((temp / 30.0) * 100)
			if expectedBrightness > 100 {
				expectedBrightness = 100
			}

			if brightness != expectedBrightness {
				t.Errorf("Expected LED brightness %d for temp %f, got %d", expectedBrightness, temp, brightness)
			}

			expectedOn := temp > 0
			if isOn != expectedOn {
				t.Errorf("Expected LED on=%t for temp %f, got %t", expectedOn, temp, isOn)
			}
		})
	}

	// Verify all temperature messages were received
	temperatureMutex.Lock()
	if len(receivedTemperatures) != len(testTemperatures) {
		t.Errorf("Expected to receive %d temperature updates, got %d", len(testTemperatures), len(receivedTemperatures))
	}
	temperatureMutex.Unlock()
}

// TestMultiDeviceCoordination tests multiple devices working together
func TestMultiDeviceCoordination(t *testing.T) {
	// Start server
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

	// Create multiple devices
	led1 := NewStatefulLEDClient("bedroom-led", "bedroom-led")
	led2 := NewStatefulLEDClient("kitchen-led", "kitchen-led")
	thermostat := NewStatefulThermostat("main-thermostat")
	tempSensor := NewStatefulTemperatureSensor("living-room-sensor")
	controller := client.NewClient("central-controller", client.NewTCPTransport())

	// Start all devices
	devices := []interface {
		Start(string) error
	}{led1, led2, thermostat, tempSensor, controller}

	for _, device := range devices {
		go func(d interface{ Start(string) error }) {
			if err := d.Start(serverAddr); err != nil {
				t.Errorf("Device failed to start: %v", err)
			}
		}(device)
	}

	// Wait for all connections
	time.Sleep(1 * time.Second)

	// Verify all devices are registered
	registeredDevices := registry.List()
	if len(registeredDevices) != 5 {
		t.Fatalf("Expected 5 devices registered, got %d", len(registeredDevices))
	}

	// Test coordinated control
	t.Run("CoordinatedLEDControl", func(t *testing.T) {
		// Turn on both LEDs with different brightness
		err := controller.SendCommand("bedroom-led", map[string]interface{}{
			"brightness": 50,
			"on":         true,
		})
		if err != nil {
			t.Fatalf("Failed to send LED command: %v", err)
		}
		if !led1.WaitForStateChange(2 * time.Second) {
			t.Error("LED1 did not update")
		}

		err = controller.SendCommand("kitchen-led", map[string]interface{}{
			"brightness": 50,
			"on":         true,
		})

		if err != nil {
			t.Fatalf("Failed to send LED command: %v", err)
		}

		if !led2.WaitForStateChange(2 * time.Second) {
			t.Error("LED2 did not update")
		}

		// Verify both LEDs have same state
		brightness1, on1 := led1.GetState()
		brightness2, on2 := led2.GetState()

		if brightness1 != 50 || brightness2 != 50 {
			t.Errorf("Expected brightness 50 for both LEDs, got %d and %d", brightness1, brightness2)
		}
		if !on1 || !on2 {
			t.Errorf("Expected both LEDs to be on, got %t and %t", on1, on2)
		}
	})

	// Test individual device queries
	t.Run("IndividualDeviceQueries", func(t *testing.T) {
		// Query each device type
		devices := map[string]string{
			"bedroom-led": "bedroom-led",
			"kitchen-led": "kitchen-led",
			"thermostat":  "thermostat",
			"temperature": "temperature",
		}

		for topic, name := range devices {
			response, err := controller.SendQuery(topic, map[string]interface{}{})
			if err != nil {
				t.Errorf("Failed to query %s: %v", name, err)
				continue
			}

			var state map[string]interface{}
			if err := json.Unmarshal(response.Payload, &state); err != nil {
				t.Errorf("Failed to unmarshal %s response: %v", name, err)
				continue
			}

			// Verify response contains expected fields based on device type
			switch topic {
			case "kitchen-led", "bedroom-led":
				if _, ok := state["brightness"]; !ok {
					t.Errorf("LED response missing brightness field")
				}
				if _, ok := state["on"]; !ok {
					t.Errorf("LED response missing on field")
				}
			case "thermostat":
				if _, ok := state["target_temp"]; !ok {
					t.Errorf("Thermostat response missing target_temp field")
				}
				if _, ok := state["mode"]; !ok {
					t.Errorf("Thermostat response missing mode field")
				}
			case "temperature":
				if _, ok := state["temperature"]; !ok {
					t.Errorf("Temperature sensor response missing temperature field")
				}
				if _, ok := state["unit"]; !ok {
					t.Errorf("Temperature sensor response missing unit field")
				}
			}
		}
	})
}

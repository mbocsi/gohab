package integration

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mbocsi/gohab/client"
	"github.com/mbocsi/gohab/proto"
)

// StatefulLEDClient represents a LED device that maintains brightness state
type StatefulLEDClient struct {
	client       *client.Client
	brightness   int
	isOn         bool
	color        string
	mu           sync.RWMutex
	stateChanged chan struct{}
	featureName  string
}

func NewStatefulLEDClient(name string, featureName string) *StatefulLEDClient {
	c := &StatefulLEDClient{
		client:       newQuietClient(name, client.NewTCPTransport()),
		featureName:  featureName,
		brightness:   0,
		isOn:         false,
		color:        "warm",
		stateChanged: make(chan struct{}, 1),
	}

	// Add LED feature with command and query support
	ledFeature := proto.Feature{
		Name: featureName,
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"brightness": {Type: "number", Range: []float64{0, 100}},
					"color":      {Type: "enum", Enum: []string{"warm", "cool", "red", "green", "blue"}},
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
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number"},
					"color":      {Type: "string"},
					"on":         {Type: "bool"},
					"power":      {Type: "number", Unit: "watts"},
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

	if color, ok := payload["color"]; ok {
		if colorStr, ok := color.(string); ok {
			if colorStr != l.color {
				l.color = colorStr
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
			// Channel full, drain one old notification and send new one
			select {
			case <-l.stateChanged:
			default:
			}
			l.stateChanged <- struct{}{}
		}
	}

	return nil
}

func (l *StatefulLEDClient) handleQuery(msg proto.Message) (any, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]interface{}{
		"brightness": l.brightness,
		"color":      l.color,
		"on":         l.isOn,
	}, nil
}

func (l *StatefulLEDClient) publishStatus() error {
	statusFn, err := l.client.GetStatusFunction(l.featureName)
	if err != nil {
		return err
	}

	l.mu.RLock()
	power := 0.0
	if l.isOn {
		power = float64(l.brightness) * 0.5 // Simulate power consumption
	}
	state := map[string]interface{}{
		"brightness": l.brightness,
		"color":      l.color,
		"on":         l.isOn,
		"power":      power,
	}
	l.mu.RUnlock()

	return statusFn(state)
}

func (l *StatefulLEDClient) GetState() (int, bool, string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.brightness, l.isOn, l.color
}

func (l *StatefulLEDClient) Start(serverAddr string) error {
	return l.client.Start(serverAddr)
}

// ResetStateChangeNotifications clears any pending notifications
func (l *StatefulLEDClient) ResetStateChangeNotifications() {
	// Drain the channel
	for {
		select {
		case <-l.stateChanged:
			// Continue draining
		default:
			return
		}
	}
}

func (l *StatefulLEDClient) WaitForStateChange(timeout time.Duration) bool {
	select {
	case <-l.stateChanged:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (l *StatefulLEDClient) GetClient() *client.Client {
	return l.client
}

func (l *StatefulLEDClient) HandleCommand(msg proto.Message) error {
	return l.handleCommand(msg)
}

// StatefulTemperatureSensor represents a temperature sensor that can simulate readings
type StatefulTemperatureSensor struct {
	client      *client.Client
	temperature float64
	featureName string
	mu          sync.RWMutex
}

func NewStatefulTemperatureSensor(name string) *StatefulTemperatureSensor {
	return NewStatefulTemperatureSensorWithFeature(name, "temperature")
}

func NewStatefulTemperatureSensorWithFeature(name string, featureName string) *StatefulTemperatureSensor {
	s := &StatefulTemperatureSensor{
		client:      newQuietClient(name, client.NewTCPTransport()),
		temperature: 20.0, // Default room temperature
		featureName: featureName,
	}

	// Add temperature sensor feature
	tempFeature := proto.Feature{
		Name: featureName,
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
	s.client.RegisterQueryHandler(featureName, s.handleQuery)

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
	dataFn, err := s.client.GetDataFunction(s.featureName)
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

func (s *StatefulTemperatureSensor) GetDataFunction(featureName string) (func(data interface{}) error, error) {
	// Return a custom data function that updates our internal temperature
	// when temperature data is published
	dataFn, err := s.client.GetDataFunction(featureName)
	if err != nil {
		return nil, err
	}
	
	return func(data interface{}) error {
		// Update internal temperature if present in data
		if dataMap, ok := data.(map[string]interface{}); ok {
			if temp, exists := dataMap["temperature"]; exists {
				if tempFloat, ok := temp.(float64); ok {
					s.SetTemperature(tempFloat)
				}
			}
		}
		// Call the original data function
		return dataFn(data)
	}, nil
}

// StatefulThermostat represents a thermostat that can control temperature and respond to commands
type StatefulThermostat struct {
	client       *client.Client
	targetTemp   float64
	currentTemp  float64
	mode         string // "heat", "cool", "auto", "off"
	status       string // "heating", "cooling", "idle", "off"
	mu           sync.RWMutex
	stateChanged chan struct{}
}

func NewStatefulThermostat(name string) *StatefulThermostat {
	t := &StatefulThermostat{
		client:       newQuietClient(name, client.NewTCPTransport()),
		targetTemp:   22.0,
		currentTemp:  20.0,
		mode:         "auto",
		status:       "idle",
		stateChanged: make(chan struct{}, 1),
	}

	// Add thermostat feature
	thermostatFeature := proto.Feature{
		Name: "hvac-control",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"target_temp": {Type: "number", Range: []float64{15, 30}, Unit: "Celsius"},
					"mode":        {Type: "enum", Enum: []string{"heat", "cool", "auto", "off"}},
				},
			},
			Query: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"target_temp":  {Type: "number", Unit: "Celsius"},
					"current_temp": {Type: "number", Unit: "Celsius"},
					"mode":         {Type: "string"},
				},
			},
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"target_temp":  {Type: "number", Unit: "Celsius"},
					"current_temp": {Type: "number", Unit: "Celsius"},
					"mode":         {Type: "string"},
					"status":       {Type: "enum", Enum: []string{"heating", "cooling", "idle", "off"}},
				},
			},
		},
	}

	t.client.AddFeature(thermostatFeature)
	t.client.RegisterCommandHandler("hvac-control", t.handleCommand)
	t.client.RegisterQueryHandler("hvac-control", t.handleQuery)

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
		// Update status based on mode and temperature difference
		t.updateStatus()
		t.publishStatus()
		select {
		case t.stateChanged <- struct{}{}:
		default:
			// Channel full, drain one old notification and send new one
			select {
			case <-t.stateChanged:
			default:
			}
			t.stateChanged <- struct{}{}
		}
	}

	return nil
}

func (t *StatefulThermostat) updateStatus() {
	// This should be called while already holding the mutex
	tempDiff := t.targetTemp - t.currentTemp
	
	switch t.mode {
	case "heat":
		if tempDiff > 0.5 {
			t.status = "heating"
		} else {
			t.status = "idle"
		}
	case "cool":
		if tempDiff < -0.5 {
			t.status = "cooling"
		} else {
			t.status = "idle"
		}
	case "auto":
		if tempDiff > 0.5 {
			t.status = "heating"
		} else if tempDiff < -0.5 {
			t.status = "cooling"
		} else {
			t.status = "idle"
		}
	case "off":
		t.status = "off"
	default:
		t.status = "idle"
	}
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
	statusFn, err := t.client.GetStatusFunction("hvac-control")
	if err != nil {
		return err
	}

	t.mu.RLock()
	state := map[string]interface{}{
		"target_temp":  t.targetTemp,
		"current_temp": t.currentTemp,
		"mode":         t.mode,
		"status":       t.status,
	}
	t.mu.RUnlock()

	return statusFn(state)
}

func (t *StatefulThermostat) GetState() (float64, float64, string, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.targetTemp, t.currentTemp, t.mode, t.status
}

func (t *StatefulThermostat) SetCurrentTemperature(temp float64) {
	t.mu.Lock()
	t.currentTemp = temp
	t.updateStatus()
	t.mu.Unlock()
	t.publishStatus()
}

func (t *StatefulThermostat) Start(serverAddr string) error {
	return t.client.Start(serverAddr)
}

// ResetStateChangeNotifications clears any pending notifications
func (t *StatefulThermostat) ResetStateChangeNotifications() {
	// Drain the channel
	for {
		select {
		case <-t.stateChanged:
			// Continue draining
		default:
			return
		}
	}
}

func (t *StatefulThermostat) WaitForStateChange(timeout time.Duration) bool {
	select {
	case <-t.stateChanged:
		return true
	case <-time.After(timeout):
		return false
	}
}
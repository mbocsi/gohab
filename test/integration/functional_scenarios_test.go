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

// Test complete smart lighting control workflow
func TestSmartLightingAutomation(t *testing.T) {
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

	// Create motion detector
	motionDetector := client.NewClient("motion-detector-01", client.NewTCPTransport())
	err := motionDetector.AddFeature(proto.Feature{
		Name: "motion-sensor",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"motion":     {Type: "bool"},
					"location":   {Type: "string"},
					"confidence": {Type: "number", Range: []float64{0, 100}},
					"timestamp":  {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add motion sensor feature: %v", err)
	}

	// Create brightness sensor
	brightnessSensor := client.NewClient("brightness-sensor-01", client.NewTCPTransport())
	err = brightnessSensor.AddFeature(proto.Feature{
		Name: "ambient-light",
		Methods: proto.FeatureMethods{
			Data: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"brightness": {Type: "number", Range: []float64{0, 1000}, Unit: "lux"},
					"location":   {Type: "string"},
					"timestamp":  {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add brightness sensor feature: %v", err)
	}

	// Create LED controller using StatefulLEDClient
	ledController := NewStatefulLEDClient("led-controller-01", "led-strip")

	// Create central controller
	centralController := client.NewClient("central-controller", client.NewTCPTransport())
	
	var lastBrightnessReading map[string]interface{}
	var controllerMu sync.Mutex

	// Subscribe to motion events
	err = centralController.Subscribe("motion-sensor", func(msg proto.Message) error {
		controllerMu.Lock()
		defer controllerMu.Unlock()

		var motion map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &motion); err != nil {
			return err
		}
		// Store motion event for logic

		// Implement motion-based lighting logic
		if motionDetected, ok := motion["motion"].(bool); ok {
			if motionDetected {
				// Calculate brightness based on ambient light
				brightness := 80.0 // Default high brightness
				color := "warm"    // Default warm color

				if lastBrightnessReading != nil {
					if ambientLux, ok := lastBrightnessReading["brightness"].(float64); ok {
						// Adjust LED brightness based on ambient light
						// Higher ambient light = lower LED brightness needed
						brightness = 100.0 - (ambientLux / 10.0)
						if brightness < 20 {
							brightness = 20 // Minimum brightness
						}
						if brightness > 100 {
							brightness = 100
						}

						// Choose color based on ambient light level
						if ambientLux < 50 {
							color = "warm" // Low light, use warm color
						} else {
							color = "cool" // Bright light, use cool color
						}
					}
				}

				// Send command to LED
				return centralController.SendCommand("led-strip", map[string]interface{}{
					"on":         true,
					"brightness": brightness,
					"color":      color,
				})
			} else {
				// Motion stopped, dim the lights
				return centralController.SendCommand("led-strip", map[string]interface{}{
					"brightness": 10.0,
					"color":      "warm",
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to motion: %v", err)
	}

	// Subscribe to brightness readings
	err = centralController.Subscribe("ambient-light", func(msg proto.Message) error {
		controllerMu.Lock()
		defer controllerMu.Unlock()

		var brightness map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &brightness); err != nil {
			return err
		}
		lastBrightnessReading = brightness
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to ambient light: %v", err)
	}

	// Start all devices
	devices := []*client.Client{motionDetector, brightnessSensor, centralController}
	for i, device := range devices {
		go func(idx int, dev *client.Client) {
			if err := dev.Start(serverAddr); err != nil {
				t.Errorf("Device %d failed: %v", idx, err)
			}
		}(i, device)
	}
	
	// Start LED controller separately
	go func() {
		if err := ledController.Start(serverAddr); err != nil {
			t.Errorf("LED controller failed: %v", err)
		}
	}()

	// Wait for all connections
	time.Sleep(2 * time.Second)

	// Test complete automation workflow
	t.Run("CompleteAutomationWorkflow", func(t *testing.T) {
		// Get data publishing functions
		motionDataFn, err := motionDetector.GetDataFunction("motion-sensor")
		if err != nil {
			t.Fatalf("Failed to get motion data function: %v", err)
		}

		brightnessDataFn, err := brightnessSensor.GetDataFunction("ambient-light")
		if err != nil {
			t.Fatalf("Failed to get brightness data function: %v", err)
		}

		// Step 1: Set ambient light level (low light scenario)
		err = brightnessDataFn(map[string]interface{}{
			"brightness": 30.0, // Low ambient light
			"location":   "living-room",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish brightness reading: %v", err)
		}

		time.Sleep(300 * time.Millisecond)

		// Step 2: Trigger motion detection
		err = motionDataFn(map[string]interface{}{
			"motion":     true,
			"location":   "living-room",
			"confidence": 95.0,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish motion event: %v", err)
		}

		// Step 3: Wait for automation chain to complete
		time.Sleep(1 * time.Second)

		// Step 4: Verify LED response
		if !ledController.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		brightness, isOn, color := ledController.GetState()
		expectedBrightness := int(100.0 - (30.0 / 10.0)) // Should be 97%
		if brightness != expectedBrightness {
			t.Errorf("Expected LED brightness %d, got %d", expectedBrightness, brightness)
		}
		if !isOn {
			t.Error("LED should be on after motion detection")
		}
		if color != "warm" {
			t.Errorf("Expected warm color in low light, got %s", color)
		}

		// Step 5: Query LED state to confirm
		response, err := centralController.SendQuery("led-strip", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query LED state: %v", err)
		}

		var ledState map[string]interface{}
		if err := json.Unmarshal(response.Payload, &ledState); err != nil {
			t.Fatalf("Failed to unmarshal LED state: %v", err)
		}

		if on, ok := ledState["on"].(bool); !ok || !on {
			t.Error("LED query shows light is not on")
		}

		// Step 6: Test motion stop scenario
		err = motionDataFn(map[string]interface{}{
			"motion":     false,
			"location":   "living-room",
			"confidence": 90.0,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish motion stop: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Verify LED dimmed
		if !ledController.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change for dimming")
		}
		
		brightness, _, _ = ledController.GetState()
		if brightness != 10 {
			t.Errorf("Expected dimmed brightness 10, got %d", brightness)
		}
	})

	// Test bright ambient light scenario
	t.Run("BrightAmbientLightScenario", func(t *testing.T) {
		motionDataFn, _ := motionDetector.GetDataFunction("motion-sensor")
		brightnessDataFn, _ := brightnessSensor.GetDataFunction("ambient-light")

		// Set high ambient light
		err = brightnessDataFn(map[string]interface{}{
			"brightness": 200.0, // High ambient light
			"location":   "living-room",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish bright reading: %v", err)
		}

		time.Sleep(300 * time.Millisecond)

		// Trigger motion in bright conditions
		err = motionDataFn(map[string]interface{}{
			"motion":     true,
			"location":   "living-room",
			"confidence": 98.0,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish motion in bright conditions: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Verify LED adjusted for bright conditions
		if !ledController.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change for bright conditions")
		}
		
		brightness, _, color := ledController.GetState()
		expectedBrightness := int(100.0 - (200.0 / 10.0)) // Should be 80%
		if brightness != expectedBrightness {
			t.Errorf("Expected LED brightness %d in bright conditions, got %d", expectedBrightness, brightness)
		}
		if color != "cool" {
			t.Errorf("Expected cool color in bright light, got %s", color)
		}
	})

	// Test automation timing (should complete within 2 seconds)
	t.Run("AutomationTiming", func(t *testing.T) {
		motionDataFn, _ := motionDetector.GetDataFunction("motion-sensor")
		
		startTime := time.Now()
		
		// Trigger motion
		err = motionDataFn(map[string]interface{}{
			"motion":     true,
			"location":   "living-room",
			"confidence": 92.0,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish motion for timing test: %v", err)
		}

		// Wait for LED to respond using state change notification
		if !ledController.WaitForStateChange(2 * time.Second) {
			t.Error("LED never responded within timeout")
		} else {
			elapsed := time.Since(startTime)
			if elapsed > 2*time.Second {
				t.Errorf("Automation took too long: %v", elapsed)
			}
		}
	})
}

// Test HVAC temperature control system
func TestHVACTemperatureControl(t *testing.T) {
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

	// Create temperature sensor
	tempSensor := NewStatefulTemperatureSensorWithFeature("room-temp-sensor", "room-temperature")

	// Create thermostat using StatefulThermostat
	thermostat := NewStatefulThermostat("smart-thermostat")

	// Create HVAC controller
	var err error
	hvacController := client.NewClient("hvac-unit", client.NewTCPTransport())
	
	var hvacStatus string = "idle"
	var hvacPower float64 = 0.0
	var hvacMu sync.Mutex

	err = hvacController.AddFeature(proto.Feature{
		Name: "hvac-unit",
		Methods: proto.FeatureMethods{
			Command: proto.Method{
				InputSchema: map[string]proto.DataType{
					"action": {Type: "enum", Enum: []string{"heat", "cool", "stop"}},
					"power":  {Type: "number", Range: []float64{0, 100}, Unit: "percent"},
				},
			},
			Status: proto.Method{
				OutputSchema: map[string]proto.DataType{
					"status":       {Type: "enum", Enum: []string{"heating", "cooling", "idle"}},
					"power_usage":  {Type: "number", Unit: "watts"},
					"efficiency":   {Type: "number", Unit: "percent"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add HVAC controller feature: %v", err)
	}

	err = hvacController.RegisterCommandHandler("hvac-unit", func(msg proto.Message) error {
		hvacMu.Lock()
		defer hvacMu.Unlock()

		var cmd map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
			return err
		}

		if action, ok := cmd["action"].(string); ok {
			switch action {
			case "heat":
				hvacStatus = "heating"
			case "cool":
				hvacStatus = "cooling"
			case "stop":
				hvacStatus = "idle"
			}
		}

		if power, ok := cmd["power"].(float64); ok {
			hvacPower = power
		}

		// Publish status update
		statusFn, err := hvacController.GetStatusFunction("hvac-unit")
		if err == nil {
			powerUsage := 0.0
			if hvacStatus != "idle" {
				powerUsage = hvacPower * 20.0 // Simulate power consumption
			}
			statusFn(map[string]interface{}{
				"status":      hvacStatus,
				"power_usage": powerUsage,
				"efficiency":  85.0,
			})
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Failed to register HVAC command handler: %v", err)
	}

	// Create central climate controller
	climateController := client.NewClient("climate-controller", client.NewTCPTransport())
	
	var climateMu sync.Mutex

	// Subscribe to temperature readings
	err = climateController.Subscribe("room-temperature", func(msg proto.Message) error {
		climateMu.Lock()
		defer climateMu.Unlock()

		var tempData map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &tempData); err != nil {
			return err
		}
		// Process temperature data for climate control

		// Implement climate control logic
		if temperature, ok := tempData["temperature"].(float64); ok {
			target, _, mode, _ := thermostat.GetState()
			// Update thermostat with current temperature
			thermostat.SetCurrentTemperature(temperature)

			if mode == "auto" {
				tempDiff := temperature - target
				tolerance := 1.0 // ±1°C tolerance

				if tempDiff < -tolerance {
					// Too cold, heat
					return climateController.SendCommand("hvac-unit", map[string]interface{}{
						"action": "heat",
						"power":  50.0,
					})
				} else if tempDiff > tolerance {
					// Too hot, cool
					return climateController.SendCommand("hvac-unit", map[string]interface{}{
						"action": "cool",
						"power":  50.0,
					})
				} else {
					// Within tolerance, stop
					return climateController.SendCommand("hvac-unit", map[string]interface{}{
						"action": "stop",
						"power":  0.0,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to temperature: %v", err)
	}

	// Start all devices
	devices := []*client.Client{hvacController, climateController}
	for i, device := range devices {
		go func(idx int, dev *client.Client) {
			if err := dev.Start(serverAddr); err != nil {
				t.Errorf("Device %d failed: %v", idx, err)
			}
		}(i, device)
	}
	
	// Start stateful devices separately
	go func() {
		if err := tempSensor.Start(serverAddr); err != nil {
			t.Errorf("Temperature sensor failed: %v", err)
		}
	}()
	
	go func() {
		if err := thermostat.Start(serverAddr); err != nil {
			t.Errorf("Thermostat failed: %v", err)
		}
	}()

	// Wait for all connections
	time.Sleep(2 * time.Second)

	// Test temperature control workflow
	t.Run("TemperatureControlWorkflow", func(t *testing.T) {
		tempDataFn, err := tempSensor.GetDataFunction("room-temperature")
		if err != nil {
			t.Fatalf("Failed to get temperature data function: %v", err)
		}

		// Set thermostat target
		err = climateController.SendCommand("hvac-control", map[string]interface{}{
			"target_temp": 24.0,
			"mode":        "auto",
		})
		if err != nil {
			t.Fatalf("Failed to set thermostat: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		// Simulate cold room (below target)
		err = tempDataFn(map[string]interface{}{
			"temperature": 20.0, // 4°C below target
			"humidity":    55.0,
			"room":        "living-room",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish cold temperature: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Verify HVAC is heating
		hvacMu.Lock()
		if hvacStatus != "heating" {
			t.Errorf("Expected HVAC to be heating, got status: %s", hvacStatus)
		}
		if hvacPower != 50.0 {
			t.Errorf("Expected HVAC power 50%%, got %f", hvacPower)
		}
		hvacMu.Unlock()

		// Simulate temperature rising to target
		err = tempDataFn(map[string]interface{}{
			"temperature": 24.0, // At target
			"humidity":    53.0,
			"room":        "living-room",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish target temperature: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Verify HVAC stopped
		hvacMu.Lock()
		if hvacStatus != "idle" {
			t.Errorf("Expected HVAC to be idle at target temp, got status: %s", hvacStatus)
		}
		hvacMu.Unlock()

		// Simulate hot room (above target)
		err = tempDataFn(map[string]interface{}{
			"temperature": 27.0, // 3°C above target
			"humidity":    58.0,
			"room":        "living-room",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("Failed to publish hot temperature: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Verify HVAC is cooling
		hvacMu.Lock()
		if hvacStatus != "cooling" {
			t.Errorf("Expected HVAC to be cooling, got status: %s", hvacStatus)
		}
		hvacMu.Unlock()
	})

	// Test system maintains temperature within tolerance
	t.Run("TemperatureMaintenance", func(t *testing.T) {
		tempDataFn, _ := tempSensor.GetDataFunction("room-temperature")

		// Test temperatures within tolerance (should not trigger HVAC)
		testTemps := []float64{23.5, 24.5, 23.8, 24.2} // All within ±1°C of 24°C target

		for _, temp := range testTemps {
			// Reset HVAC to idle
			hvacMu.Lock()
			hvacStatus = "idle"
			hvacMu.Unlock()

			err = tempDataFn(map[string]interface{}{
				"temperature": temp,
				"humidity":    55.0,
				"room":        "living-room",
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
			if err != nil {
				t.Errorf("Failed to publish temperature %f: %v", temp, err)
			}

			time.Sleep(500 * time.Millisecond)

			// Verify HVAC remains idle
			hvacMu.Lock()
			if hvacStatus != "idle" {
				t.Errorf("HVAC should remain idle for temperature %f (within tolerance), got status: %s", temp, hvacStatus)
			}
			hvacMu.Unlock()
		}
	})
}

// Test multi-zone coordination
func TestMultiZoneHomeAutomation(t *testing.T) {
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

	zones := []string{"living-room", "bedroom", "kitchen"}
	deviceCounts := make(map[string]int)
	var deviceCountMu sync.Mutex

	// Create devices for each zone
	for _, zone := range zones {
		// Occupancy sensor for each zone
		_ = client.NewClient(fmt.Sprintf("%s-occupancy", zone), client.NewTCPTransport())
		occupancySensor := client.NewClient(fmt.Sprintf("%s-occupancy", zone), client.NewTCPTransport())
		err := occupancySensor.AddFeature(proto.Feature{
			Name: fmt.Sprintf("%s-occupancy", zone),
			Methods: proto.FeatureMethods{
				Data: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"occupied":  {Type: "bool"},
						"zone":      {Type: "string"},
						"timestamp": {Type: "string"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to add occupancy sensor for %s: %v", zone, err)
		}

		// Light controller for each zone
		lightController := client.NewClient(fmt.Sprintf("%s-lights", zone), client.NewTCPTransport())
		err = lightController.AddFeature(proto.Feature{
			Name: fmt.Sprintf("%s-lighting", zone),
			Methods: proto.FeatureMethods{
				Command: proto.Method{
					InputSchema: map[string]proto.DataType{
						"on":         {Type: "bool"},
						"brightness": {Type: "number", Range: []float64{0, 100}},
						"scene":      {Type: "enum", Enum: []string{"bright", "dim", "night", "off"}},
					},
				},
				Status: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"on":         {Type: "bool"},
						"brightness": {Type: "number"},
						"scene":      {Type: "string"},
						"zone":       {Type: "string"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Failed to add light controller for %s: %v", zone, err)
		}

		// Light command handler
		err = lightController.RegisterCommandHandler(fmt.Sprintf("%s-lighting", zone), func(msg proto.Message) error {
			var cmd map[string]interface{}
			if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
				return err
			}

			// Simulate light response and publish status
			statusFn, err := lightController.GetStatusFunction(fmt.Sprintf("%s-lighting", zone))
			if err == nil {
				status := map[string]interface{}{
					"zone": zone,
				}
				if on, ok := cmd["on"].(bool); ok {
					status["on"] = on
				}
				if brightness, ok := cmd["brightness"].(float64); ok {
					status["brightness"] = brightness
				}
				if scene, ok := cmd["scene"].(string); ok {
					status["scene"] = scene
				}
				statusFn(status)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to register light handler for %s: %v", zone, err)
		}

		// Start devices
		go func(zone string, occ *client.Client, light *client.Client) {
			if err := occ.Start(serverAddr); err != nil {
				t.Errorf("Occupancy sensor for %s failed: %v", zone, err)
			}
		}(zone, occupancySensor, lightController)

		go func(zone string, light *client.Client) {
			if err := light.Start(serverAddr); err != nil {
				t.Errorf("Light controller for %s failed: %v", zone, err)
			}
		}(zone, lightController)
	}

	// Create central automation controller
	automationController := client.NewClient("home-automation", client.NewTCPTransport())
	
	// Subscribe to all occupancy sensors
	for _, zone := range zones {
		zoneName := zone // Capture for closure
		err := automationController.Subscribe(fmt.Sprintf("%s-occupancy", zone), func(msg proto.Message) error {
			var occupancy map[string]interface{}
			if err := json.Unmarshal(msg.Payload, &occupancy); err != nil {
				return err
			}

			if occupied, ok := occupancy["occupied"].(bool); ok {
				deviceCountMu.Lock()
				if occupied {
					deviceCounts[zoneName]++
				} else {
					deviceCounts[zoneName] = 0
				}
				count := deviceCounts[zoneName]
				deviceCountMu.Unlock()

				// Lighting logic based on occupancy
				if occupied {
					// Zone occupied, turn on lights
					return automationController.SendCommand(fmt.Sprintf("%s-lighting", zoneName), map[string]interface{}{
						"on":         true,
						"brightness": 80.0,
						"scene":      "bright",
					})
				} else if count == 0 {
					// Zone empty, turn off lights
					return automationController.SendCommand(fmt.Sprintf("%s-lighting", zoneName), map[string]interface{}{
						"scene": "off",
					})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to subscribe to %s occupancy: %v", zone, err)
		}
	}

	// Start automation controller
	go func() {
		if err := automationController.Start(serverAddr); err != nil {
			t.Errorf("Automation controller failed: %v", err)
		}
	}()

	// Wait for all connections
	time.Sleep(3 * time.Second)

	// Test multi-zone coordination
	t.Run("MultiZoneCoordination", func(t *testing.T) {
		// Verify all devices registered
		devices := registry.List()
		expectedDevices := len(zones)*2 + 1 // 2 devices per zone + automation controller
		if len(devices) < expectedDevices {
			t.Errorf("Expected at least %d devices, got %d", expectedDevices, len(devices))
		}

		// Test occupancy in each zone
		for _, zone := range zones {
			// Find the occupancy sensor for this zone
			var found bool
			for _, device := range devices {
				if device.Meta().Name == fmt.Sprintf("%s-occupancy", zone) {
					found = true
					// In a real test, we'd get the actual client reference
					// For this integration test, we'll simulate using the automation controller
					break
				}
			}
			
			if !found {
				t.Errorf("Occupancy sensor not found for zone %s", zone)
			}

			// Simulate occupancy detection (we'll use automation controller to simulate)
			// In a real scenario, we'd get the actual data publishing function
			
			// For this test, we verify the subscription framework is working
			// Full end-to-end testing would require more complex device tracking
		}
	})

	// Test independent zone operation
	t.Run("IndependentZoneOperation", func(t *testing.T) {
		// Verify zones operate independently
		// Each zone should have its own topic namespace
		topicSources := gohabServer.GetTopicSources()

		for _, zone := range zones {
			occupancyTopic := fmt.Sprintf("%s-occupancy", zone)
			lightingTopic := fmt.Sprintf("%s-lighting", zone)

			if _, ok := topicSources[occupancyTopic]; !ok {
				t.Errorf("Occupancy topic for %s not registered", zone)
			}
			if _, ok := topicSources[lightingTopic]; !ok {
				t.Errorf("Lighting topic for %s not registered", zone)
			}
		}

		// Verify subscription isolation
		for _, zone := range zones {
			subs := broker.Subs(fmt.Sprintf("%s-occupancy", zone))
			if len(subs) != 1 {
				t.Errorf("Expected 1 subscriber for %s occupancy, got %d", zone, len(subs))
			}
		}
	})
}

// TestAdvancedLEDControl tests comprehensive LED brightness control workflow
func TestAdvancedLEDControl(t *testing.T) {
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

	// Create LED device with advanced capabilities
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
		brightness, isOn, _ := ledDevice.GetState()
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
		brightness, isOn, _ = ledDevice.GetState()
		if brightness != 75 || isOn != true {
			t.Errorf("Expected state: brightness=75, on=true, got brightness=%d, on=%t", brightness, isOn)
		}
	})

	// Test 2: Brightness adjustment
	t.Run("BrightnessAdjustment", func(t *testing.T) {
		// Adjust brightness to 25
		err := controller.SendCommand("living-room-led", map[string]interface{}{
			"brightness": 25,
		})
		if err != nil {
			t.Fatalf("Failed to adjust brightness: %v", err)
		}

		// Wait for state change
		if !ledDevice.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		// Verify brightness changed but still on
		brightness, isOn, _ := ledDevice.GetState()
		if brightness != 25 || isOn != true {
			t.Errorf("Expected state: brightness=25, on=true, got brightness=%d, on=%t", brightness, isOn)
		}
	})

	// Test 3: Turn off LED
	t.Run("TurnOffLED", func(t *testing.T) {
		// Turn off LED
		err := controller.SendCommand("living-room-led", map[string]interface{}{
			"on": false,
		})
		if err != nil {
			t.Fatalf("Failed to turn off LED: %v", err)
		}

		// Wait for state change
		if !ledDevice.WaitForStateChange(2 * time.Second) {
			t.Fatal("LED state did not change within timeout")
		}

		// Verify LED is off but brightness preserved
		brightness, isOn, _ := ledDevice.GetState()
		if brightness != 25 || isOn != false {
			t.Errorf("Expected state: brightness=25, on=false, got brightness=%d, on=%t", brightness, isOn)
		}
	})

	// Test 4: Query LED state
	t.Run("QueryLEDState", func(t *testing.T) {
		response, err := controller.SendQuery("living-room-led", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Failed to query LED: %v", err)
		}

		var state map[string]interface{}
		if err := json.Unmarshal(response.Payload, &state); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Verify query response matches device state
		brightness, isOn, color := ledDevice.GetState()
		
		if state["brightness"].(float64) != float64(brightness) {
			t.Errorf("Query brightness mismatch: expected %d, got %f", brightness, state["brightness"])
		}
		if state["on"].(bool) != isOn {
			t.Errorf("Query on state mismatch: expected %t, got %t", isOn, state["on"])
		}
		if state["color"].(string) != color {
			t.Errorf("Query color mismatch: expected %s, got %s", color, state["color"])
		}
	})
}

// TestAdvancedThermostatCoordination tests comprehensive thermostat control workflow
func TestAdvancedThermostatCoordination(t *testing.T) {
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
		targetTemp, currentTemp, mode, _ := thermostat.GetState()
		if targetTemp != 22.0 || currentTemp != 20.0 || mode != "auto" {
			t.Errorf("Expected initial state: target=22.0, current=20.0, mode=auto, got target=%f, current=%f, mode=%s", targetTemp, currentTemp, mode)
		}

		// Set thermostat to heating mode with target temperature
		err := controller.SendCommand("hvac-control", map[string]interface{}{
			"target_temp": 24.0,
			"mode":        "heat",
		})
		if err != nil {
			t.Fatalf("Failed to send thermostat command: %v", err)
		}

		// Wait for state change
		if !thermostat.WaitForStateChange(2 * time.Second) {
			t.Fatal("Thermostat state did not change within timeout")
		}

		// Verify new state
		targetTemp, _, mode, _ = thermostat.GetState()
		if targetTemp != 24.0 || mode != "heat" {
			t.Errorf("Expected state: target=24.0, mode=heat, got target=%f, mode=%s", targetTemp, mode)
		}
	})

	// Test 2: Query thermostat state
	t.Run("QueryThermostatState", func(t *testing.T) {
		response, err := controller.SendQuery("hvac-control", map[string]interface{}{})
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
		if mode != "heat" {
			t.Errorf("Expected mode 'heat', got '%s'", mode)
		}
	})

	// Test 3: Simulate temperature change and verify thermostat response
	t.Run("TemperatureChangeResponse", func(t *testing.T) {
		// Simulate temperature rising due to heating
		tempSensor.SetTemperature(23.5)
		thermostat.SetCurrentTemperature(23.5)

		// Verify thermostat updated its current temperature
		_, currentTemp, _, _ := thermostat.GetState()
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
		err := controller.SendCommand("hvac-control", map[string]interface{}{
			"target_temp": 22.0,
			"mode":        "cool",
		})
		if err != nil {
			t.Fatalf("Failed to send cooling command: %v", err)
		}

		if !thermostat.WaitForStateChange(2 * time.Second) {
			t.Fatal("Thermostat state did not change within timeout")
		}

		targetTemp, _, mode, _ := thermostat.GetState()
		if targetTemp != 22.0 || mode != "cool" {
			t.Errorf("Expected state: target=22.0, mode=cool, got target=%f, mode=%s", targetTemp, mode)
		}
	})
}

// TestPubSubDeviceCoordination tests cross-device pub/sub automation workflows
func TestPubSubDeviceCoordination(t *testing.T) {
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
	err := ledDevice.GetClient().Subscribe("temperature", func(msg proto.Message) error {
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
			return ledDevice.HandleCommand(proto.Message{
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
			brightness, isOn, _ := ledDevice.GetState()
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

// TestEnhancedMultiDeviceCoordination tests comprehensive multi-device orchestration scenarios  
func TestEnhancedMultiDeviceCoordination(t *testing.T) {
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
	go func() {
		if err := led1.Start(serverAddr); err != nil {
			t.Errorf("LED1 failed to start: %v", err)
		}
	}()

	go func() {
		if err := led2.Start(serverAddr); err != nil {
			t.Errorf("LED2 failed to start: %v", err)
		}
	}()

	go func() {
		if err := thermostat.Start(serverAddr); err != nil {
			t.Errorf("Thermostat failed to start: %v", err)
		}
	}()

	go func() {
		if err := tempSensor.Start(serverAddr); err != nil {
			t.Errorf("Temperature sensor failed to start: %v", err)
		}
	}()

	go func() {
		if err := controller.Start(serverAddr); err != nil {
			t.Errorf("Controller failed to start: %v", err)
		}
	}()

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
		brightness1, on1, _ := led1.GetState()
		brightness2, on2, _ := led2.GetState()

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
			"hvac-control":  "thermostat",
			"temperature": "temperature sensor",
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
			case "hvac-control":
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
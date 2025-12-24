# GoHab - Home Automation Server

GoHab is a home automation server implementing a message-based architecture with device features and pub/sub messaging for managing IoT devices and sensors.

## Installation

```bash
go get github.com/mbocsi/gohab
```

## Quick Start

### Running the Server

Build and run the GoHab server:

```bash
make server
./bin/server
```

- Web interface: http://localhost:8080
- TCP transport: localhost:8888

### Creating a Client

Import the GoHab client library in your Go project:

```go
import (
    "github.com/mbocsi/gohab/client"
    "github.com/mbocsi/gohab/proto"
)
```

Basic client setup:

```go
// Create TCP transport and client
tcp := client.NewTCPTransport()
c := client.NewClient("my-device", tcp)

// Define a feature
feature := proto.Feature{
    Name:        "temperature",
    Description: "Temperature sensor",
    Methods: proto.FeatureMethods{
        Data: proto.Method{
            Description: "Temperature readings",
            OutputSchema: map[string]proto.DataType{
                "temperature": {Type: "number", Unit: "Celsius"},
                "timestamp":   {Type: "string"},
            },
        },
    },
}

// Add feature and start
c.AddFeature(feature)
c.Start("localhost:8888")
```

## Architecture

### Core Components

**Server:**
- `GohabServer` - Main server coordinating all components
- `Broker` - Pub/sub message broker for topic-based communication
- `DeviceRegistry` - Manages connected devices and their metadata
- `Transport` - Abstraction for client connections (TCP, WebSocket)

**Client:**
- `Client` - Main client with capability management
- `Transport` - Client transport abstraction (TCP implementation)
- Device features define supported actions/data

### Message Protocol

JSON-based messages with types:
- `identify` - Device registration with features
- `command` - Control device actions
- `query` - Request information from devices
- `response` - Reply to queries
- `data` - Publish sensor readings
- `status` - Report device status
- `subscribe` - Subscribe to topics

### Features System

Devices declare features during identification. Each feature can support four method types:

- **Data** - Publish sensor readings or state updates
- **Status** - Report device health/status
- **Command** - Accept control commands
- **Query** - Respond to information requests

## Client API

### Creating a Client

```go
// Create TCP transport and client
tcp := client.NewTCPTransport()
client := client.NewClient("device-name", tcp)

// Connect to server via TCP
err := client.Start("localhost:8888")
```

### WebSocket Transport

As an alternative to TCP, clients can also use WebSocket transport:

```go
// Create WebSocket transport and client
ws := client.NewWebSocketTransport()
client := client.NewClient("device-name", ws)

// Connect to server via WebSocket
err := client.Start("ws://localhost:8889")
```

### Defining Features

```go
feature := proto.Feature{
    Name:        "led_strip",
    Description: "RGB LED strip controller",
    Methods: proto.FeatureMethods{
        Command: proto.Method{
            Description: "Set LED color and brightness",
            InputSchema: map[string]proto.DataType{
                "color":      {Type: "string", Description: "Hex color code"},
                "brightness": {Type: "number", Range: []float64{0, 100}},
            },
        },
        Status: proto.Method{
            Description: "LED strip status",
            OutputSchema: map[string]proto.DataType{
                "status": {Type: "enum", Enum: []string{"on", "off", "error"}},
            },
        },
    },
}

client.AddFeature(feature)
```

### Handling Commands

```go
client.RegisterCommandHandler("led_strip", func(msg proto.Message) error {
    var cmd struct {
        Color      string  `json:"color"`
        Brightness float64 `json:"brightness"`
    }
    
    if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
        return err
    }
    
    // Apply LED settings
    setLEDColor(cmd.Color, cmd.Brightness)
    return nil
})
```

### Publishing Data

```go
// Get data function for a feature
dataFn, err := client.GetDataFunction("temperature")
if err != nil {
    panic(err)
}

// Publish temperature data
dataFn(map[string]interface{}{
    "temperature": 23.5,
    "timestamp":   time.Now().Format(time.RFC3339),
})
```

### Responding to Queries

```go
client.RegisterQueryHandler("temperature", func(msg proto.Message) (any, error) {
    currentTemp := readTemperatureSensor()
    return map[string]interface{}{
        "temperature": currentTemp,
        "timestamp":   time.Now().Format(time.RFC3339),
    }, nil
})
```

### Subscribing to Topics

```go
client.Subscribe("temperature", func(msg proto.Message) error {
    fmt.Printf("Received temperature update: %s\n", string(msg.Payload))
    return nil
})
```

### Sending Commands to Other Devices

```go
client.SendCommand("display_brightness", map[string]interface{}{
    "brightness": 75.0,
})
```

### Querying Other Devices

```go
response, err := client.SendQuery("temperature", nil)
if err != nil {
    log.Printf("Query failed: %v", err)
    return
}

fmt.Printf("Temperature response: %s\n", string(response.Payload))
```

## Data Types

The capability system supports typed schemas for validation:

```go
proto.DataType{
    Type:        "number",           // number, string, bool, enum, object
    Unit:        "Celsius",          // Optional unit
    Description: "Temperature value", // Optional description
    Range:       []float64{-40, 85}, // Optional range for numbers
    Enum:        []string{"on", "off"}, // Required for enum type
    Optional:    false,              // Whether field is optional
}
```

## Web Interface

The server provides a web interface at http://localhost:8080 showing:
- Connected devices and their capabilities
- Available features (topics) and their sources
- Transport connections and status
- Real-time updates via Server-Sent Events

## Development

### Building from Source

```bash
# Build server
make server

# Build example clients (for reference)
make client1 client2 client3

# Build everything
make all

# Clean build artifacts
make clean
```

### Project Structure

```
├── cmd/           # Server and example clients
├── server/        # Server components
├── client/        # Client library (import this)
├── proto/         # Message and capability definitions
├── services/      # Server services
├── web/           # Web interface
├── templates/     # HTML templates
└── assets/        # Static web assets
```

### Example Implementation

Here's a complete example of a temperature sensor client:

```go
package main

import (
    "log"
    "math/rand"
    "time"
    
    "github.com/mbocsi/gohab/client"
    "github.com/mbocsi/gohab/proto"
)

func main() {
    // Create client
    tcp := client.NewTCPTransport()
    c := client.NewClient("temp-sensor-01", tcp)
    
    // Define temperature feature
    tempFeature := proto.Feature{
        Name:        "temperature",
        Description: "Indoor temperature sensor",
        Methods: proto.FeatureMethods{
            Data: proto.Method{
                Description: "Temperature readings",
                OutputSchema: map[string]proto.DataType{
                    "temperature": {Type: "number", Unit: "Celsius"},
                    "humidity":    {Type: "number", Unit: "percent"},
                    "timestamp":   {Type: "string"},
                },
            },
            Query: proto.Method{
                Description: "Get current temperature",
                OutputSchema: map[string]proto.DataType{
                    "temperature": {Type: "number", Unit: "Celsius"},
                    "humidity":    {Type: "number", Unit: "percent"},
                },
            },
        },
    }
    
    // Add feature
    c.AddFeature(tempFeature)
    
    // Handle queries
    c.RegisterQueryHandler("temperature", func(msg proto.Message) (any, error) {
        return map[string]interface{}{
            "temperature": readTemperature(),
            "humidity":    readHumidity(),
        }, nil
    })
    
    // Get data publishing function
    dataFn, _ := c.GetDataFunction("temperature")
    
    // Start client
    go c.Start("localhost:8888")
    
    // Publish data every 30 seconds
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        dataFn(map[string]interface{}{
            "temperature": readTemperature(),
            "humidity":    readHumidity(),
            "timestamp":   time.Now().Format(time.RFC3339),
        })
    }
}

func readTemperature() float64 {
    return rand.Float64()*10 + 20 // Mock sensor
}

func readHumidity() float64 {
    return rand.Float64()*40 + 40 // Mock sensor
}
```

## License

This project is licensed under the MIT License.
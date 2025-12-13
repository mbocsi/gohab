# GoHab IoT System - Integration Test Requirements

## Overview

This document outlines comprehensive integration test requirements for the GoHab IoT home automation system. GoHab implements a message-based pub/sub architecture supporting multiple transport protocols and device capabilities.

## System Architecture

### Core Components
- **GohabServer**: Main server coordinating all components
- **Broker**: Pub/sub message broker for topic-based communication  
- **DeviceRegistry**: Manages connected devices and their metadata
- **Transport Layer**: Supports TCP, WebSocket, and LoRa protocols
- **Client Library**: Device implementation with capability management

### Message Protocol
- **Types**: `identify`, `command`, `query`, `response`, `data`, `status`, `subscribe`
- **Format**: JSON-based with flexible payload schemas
- **Routing**: Topic-based (e.g., "temperature", "led/brightness")

### Device Capabilities
Devices declare capabilities during identification with supported methods:
- `data`: Publish sensor readings or state
- `status`: Report device status  
- `command`: Accept control commands
- `query`: Respond to information requests

## Test Categories and Requirements

### 1. Transport Layer Integration Tests

#### 1.1 TCP Transport
**Scope**: Test TCP client-server communication
- **Test**: Single TCP client connection and identification
  - **Input**: TCP client with basic feature
  - **Expected**: Device registered in registry
- **Test**: Multiple TCP clients simultaneously
  - **Input**: 5+ TCP clients with different features
  - **Expected**: All devices registered with unique IDs
- **Test**: TCP client reconnection after connection loss
  - **Input**: Simulate network interruption
  - **Expected**: Client reconnects and re-registers

#### 1.2 WebSocket Transport
**Scope**: Test WebSocket client-server communication
- **Test**: Single WebSocket client connection
  - **Input**: WS client connecting to ws://host:port/ws
  - **Expected**: Device registered successfully
- **Test**: WebSocket message streaming
  - **Input**: High-frequency data publishing
  - **Expected**: No message loss or corruption
- **Test**: WebSocket ping/pong keepalive
  - **Input**: Long-running connection
  - **Expected**: Connection remains stable

#### 1.3 LoRa Transport (Future)
**Scope**: Test LoRa wireless communication
- **Test**: LoRa device registration
  - **Input**: LoRa client with SX1276 radio
  - **Expected**: Device registered via radio protocol
- **Test**: LoRa range and reliability
  - **Input**: Varying distances and signal conditions
  - **Expected**: Stable communication within range

#### 1.4 Mixed Transport Scenarios
**Scope**: Test multiple transport types simultaneously
- **Test**: TCP + WebSocket clients on same server
  - **Input**: Mix of TCP and WS clients
  - **Expected**: All transports function independently
- **Test**: Cross-transport communication
  - **Input**: TCP client publishes, WS client subscribes
  - **Expected**: Messages delivered across transports

### 2. Device Lifecycle Integration Tests

#### 2.1 Device Identification and Registration
**Scope**: Test device onboarding process
- **Test**: Basic device identification
  - **Input**: Client with `IdentifyPayload` including features
  - **Expected**: Server assigns ID and returns `IdAckPayload`
- **Test**: Device with invalid capabilities
  - **Input**: Client with malformed feature definitions
  - **Expected**: Registration rejected with error
- **Test**: Duplicate device names
  - **Input**: Two clients with same proposed name
  - **Expected**: Both registered with unique IDs
- **Test**: Device re-identification after restart
  - **Input**: Client reconnects with same features
  - **Expected**: Recognized as same device or new registration

#### 2.2 Feature Validation
**Scope**: Test capability schema validation
- **Test**: Valid feature schemas
  - **Input**: Features with proper DataType definitions
  - **Expected**: Features accepted and topic routes created
- **Test**: Invalid data types
  - **Input**: Feature with unsupported data type
  - **Expected**: Validation error returned
- **Test**: Missing required method schemas
  - **Input**: Command method without InputSchema
  - **Expected**: Feature validation fails
- **Test**: Enum validation
  - **Input**: Enum type with empty enum values
  - **Expected**: Schema validation error

#### 2.3 Device Disconnection and Cleanup
**Scope**: Test device lifecycle management
- **Test**: Graceful disconnection
  - **Input**: Client closes connection normally
  - **Expected**: Device removed from registry and topic subscriptions
- **Test**: Unexpected disconnection
  - **Input**: Network failure or client crash
  - **Expected**: Server detects timeout and cleans up device
- **Test**: Subscription cleanup on disconnect
  - **Input**: Device with active subscriptions disconnects
  - **Expected**: All subscriptions removed from broker

### 3. Message Protocol Integration Tests

#### 3.1 Command Messages
**Scope**: Test device command execution
- **Test**: Simple command execution
  - **Input**: Controller sends command to LED device
  - **Expected**: LED device receives and executes command
- **Test**: Command with invalid parameters
  - **Input**: Command with wrong data types or missing fields
  - **Expected**: Command rejected with error
- **Test**: Command to non-existent device
  - **Input**: Command to unregistered device ID
  - **Expected**: Error response returned
- **Test**: Batch command execution
  - **Input**: Multiple commands sent rapidly
  - **Expected**: All commands processed in order

#### 3.2 Query Messages
**Scope**: Test device information queries
- **Test**: Basic device query
  - **Input**: Query sent to temperature sensor
  - **Expected**: Current temperature value returned
- **Test**: Query timeout handling
  - **Input**: Query to unresponsive device
  - **Expected**: Timeout error after configured interval
- **Test**: Query response validation
  - **Input**: Query expecting specific output schema
  - **Expected**: Response matches declared schema
- **Test**: Concurrent queries
  - **Input**: Multiple simultaneous queries to same device
  - **Expected**: All queries answered correctly

#### 3.3 Data Publishing
**Scope**: Test sensor data publication
- **Test**: Periodic data publishing
  - **Input**: Temperature sensor publishing readings every 5 seconds
  - **Expected**: Data messages delivered to subscribers
- **Test**: High-frequency data streams
  - **Input**: Sensor publishing at 100Hz
  - **Expected**: No message loss or broker overload
- **Test**: Data schema validation
  - **Input**: Published data matching feature OutputSchema
  - **Expected**: Data accepted and forwarded
- **Test**: Large payload handling
  - **Input**: Device publishing large JSON payloads
  - **Expected**: Messages handled without truncation

#### 3.4 Status Reporting
**Scope**: Test device status management
- **Test**: Status update propagation
  - **Input**: Device publishes status change
  - **Expected**: Status reflected in registry and forwarded to subscribers
- **Test**: Status query vs. published status consistency
  - **Input**: Query device status after status update
  - **Expected**: Queried status matches last published status
- **Test**: Status enumeration validation
  - **Input**: Status with enum constraints
  - **Expected**: Only valid enum values accepted

### 4. Pub/Sub Broker Integration Tests

#### 4.1 Subscription Management
**Scope**: Test topic subscription lifecycle
- **Test**: Basic subscription setup
  - **Input**: Client subscribes to "temperature" topic
  - **Expected**: Client added to topic subscriber list
- **Test**: Multiple subscribers per topic
  - **Input**: 10 clients subscribe to same topic
  - **Expected**: All clients receive published messages
- **Test**: Subscription to non-existent topic
  - **Input**: Subscribe to topic with no publishers
  - **Expected**: Subscription registered, no messages received
- **Test**: Unsubscription
  - **Input**: Client unsubscribes from topic
  - **Expected**: Client no longer receives messages for that topic

#### 4.2 Message Routing
**Scope**: Test message delivery patterns
- **Test**: One-to-many broadcasting
  - **Input**: One publisher, multiple subscribers
  - **Expected**: All subscribers receive copies of each message
- **Test**: Topic isolation
  - **Input**: Messages on different topics
  - **Expected**: Subscribers only receive messages for subscribed topics
- **Test**: Message ordering
  - **Input**: Rapid sequence of messages on same topic
  - **Expected**: Messages delivered in published order
- **Test**: Cross-device communication
  - **Input**: Device A publishes, Device B subscribes and reacts
  - **Expected**: Pub/sub enables device coordination

#### 4.3 Subscription Patterns
**Scope**: Test advanced subscription scenarios
- **Test**: Wildcard subscriptions (if supported)
  - **Input**: Subscribe to "sensor/*" pattern
  - **Expected**: Receive messages from sensor/temp, sensor/humidity, etc.
- **Test**: Late subscription
  - **Input**: Subscribe after messages already published
  - **Expected**: No retroactive message delivery
- **Test**: Subscription persistence across reconnection
  - **Input**: Client reconnects with same subscriptions
  - **Expected**: Subscriptions restored automatically

### 5. End-to-End Device Scenarios

#### 5.1 Smart Lighting Control
**Scope**: Complete LED control workflow
- **Devices**: LED controller, brightness sensor, motion detector, central controller
- **Test Scenario**:
  1. Motion detector publishes motion event
  2. Central controller receives motion, calculates brightness based on ambient light
  3. Controller sends command to LED with brightness value
  4. LED executes command and reports status
  5. Brightness sensor confirms light level change
- **Expected**: Complete automation chain executes within 2 seconds

#### 5.2 HVAC Temperature Control
**Scope**: Thermostat and sensor coordination
- **Devices**: Thermostat, temperature sensor, humidity sensor, HVAC controller
- **Test Scenario**:
  1. Temperature sensor publishes readings every 30 seconds
  2. Thermostat subscribes to temperature and adjusts target
  3. HVAC controller receives commands and reports status
  4. System maintains temperature within ±1°C of target
- **Expected**: Stable temperature control with status feedback

#### 5.3 Multi-Zone Home Automation
**Scope**: Complex multi-device coordination
- **Devices**: 5+ rooms, each with lights, sensors, and actuators
- **Test Scenario**:
  1. Occupancy sensors detect presence in each room
  2. Central controller coordinates lighting based on occupancy
  3. Temperature control adjusts per zone
  4. Security system monitors all sensors
- **Expected**: All zones operate independently and report to central system

#### 5.4 Energy Management
**Scope**: Device power monitoring and control
- **Devices**: Smart outlets, power meters, load controllers
- **Test Scenario**:
  1. Power meters publish consumption data
  2. Load controller subscribes and implements demand response
  3. Smart outlets can be remotely controlled for load shedding
  4. Central dashboard displays real-time energy usage
- **Expected**: Power monitoring with automated load management

### 6. Performance and Scalability Tests

#### 6.1 High Device Count
**Scope**: Test system capacity limits
- **Test**: 100+ concurrent devices
  - **Input**: Large number of simulated devices
  - **Expected**: All devices maintain stable connections
- **Test**: Device registration burst
  - **Input**: 50 devices connecting simultaneously
  - **Expected**: All registrations complete within 30 seconds
- **Test**: Memory usage scaling
  - **Input**: Increasing device count
  - **Expected**: Memory usage scales linearly, no memory leaks

#### 6.2 Message Throughput
**Scope**: Test broker performance under load
- **Test**: High-frequency publishing
  - **Input**: 1000 messages/second across all topics
  - **Expected**: All messages delivered with <100ms latency
- **Test**: Large subscription lists
  - **Input**: 1000 subscribers to single topic
  - **Expected**: Message fan-out completes in <1 second
- **Test**: Mixed workload
  - **Input**: Combination of commands, queries, data, status messages
  - **Expected**: All message types handled without interference

#### 6.3 Resource Management
**Scope**: Test server resource utilization
- **Test**: Connection pooling efficiency
  - **Input**: Many short-lived connections
  - **Expected**: Server handles connection churn gracefully
- **Test**: Broker memory usage
  - **Input**: Long-running test with continuous messaging
  - **Expected**: No memory leaks in subscription management
- **Test**: CPU usage under load
  - **Input**: Maximum realistic message load
  - **Expected**: CPU usage remains under 80% on target hardware

### 7. Error Handling and Recovery Tests

#### 7.1 Network Failure Scenarios
**Scope**: Test resilience to network issues
- **Test**: Partial network partition
  - **Input**: Some devices disconnect while others remain
  - **Expected**: Remaining devices continue normal operation
- **Test**: Server restart with active clients
  - **Input**: Restart server while clients connected
  - **Expected**: Clients detect disconnect and automatically reconnect
- **Test**: Message delivery during reconnection
  - **Input**: Messages sent during client reconnection window
  - **Expected**: No message loss or duplication

#### 7.2 Malformed Message Handling
**Scope**: Test protocol robustness
- **Test**: Invalid JSON messages
  - **Input**: Corrupted or malformed JSON
  - **Expected**: Server logs error and continues operation
- **Test**: Unknown message types
  - **Input**: Messages with unsupported type field
  - **Expected**: Error response sent, connection maintained
- **Test**: Schema violations
  - **Input**: Messages not matching declared schemas
  - **Expected**: Validation error returned to sender

#### 7.3 Resource Exhaustion
**Scope**: Test behavior under resource constraints
- **Test**: Maximum connection limit
  - **Input**: Exceed configured connection limit
  - **Expected**: New connections rejected gracefully
- **Test**: Message queue overflow
  - **Input**: Flood of messages to slow consumer
  - **Expected**: Queue management prevents memory exhaustion
- **Test**: Storage capacity limits
  - **Input**: Excessive device registration data
  - **Expected**: Appropriate error handling when limits reached

### 8. Security Integration Tests

#### 8.1 Authentication and Authorization (Future)
**Scope**: Test access control mechanisms
- **Test**: Unauthenticated device connection
  - **Input**: Device without credentials
  - **Expected**: Connection rejected
- **Test**: Command authorization
  - **Input**: Device attempts unauthorized command
  - **Expected**: Command rejected with permission error
- **Test**: Topic access control
  - **Input**: Device subscribes to restricted topic
  - **Expected**: Subscription denied

#### 8.2 Message Integrity
**Scope**: Test message validation and security
- **Test**: Message tampering detection
  - **Input**: Messages with modified signatures
  - **Expected**: Tampered messages rejected
- **Test**: Replay attack prevention
  - **Input**: Duplicate messages with same timestamp
  - **Expected**: Replay messages detected and dropped

### 9. Web Interface Integration Tests

#### 9.1 Real-time Updates
**Scope**: Test HTMX web interface
- **Test**: Device status display
  - **Input**: Device status changes
  - **Expected**: Web interface updates in real-time via SSE
- **Test**: Command execution from web UI
  - **Input**: User clicks device control button
  - **Expected**: Command sent to device and status updated
- **Test**: Device list management
  - **Input**: Devices connect/disconnect
  - **Expected**: Device list updates automatically

#### 9.2 Feature Management
**Scope**: Test feature display and interaction
- **Test**: Feature discovery
  - **Input**: New device with unknown features
  - **Expected**: Features appear in web interface dynamically
- **Test**: Schema-driven UI generation
  - **Input**: Feature with complex input schema
  - **Expected**: Appropriate form controls generated automatically

### 10. Configuration and Deployment Tests

#### 10.1 Server Configuration
**Scope**: Test server startup and configuration
- **Test**: Multiple transport configuration
  - **Input**: Server configured with TCP, WS, and LoRa transports
  - **Expected**: All transports start and accept connections
- **Test**: Port binding conflicts
  - **Input**: Attempt to bind to occupied port
  - **Expected**: Graceful error handling and alternative port selection

#### 10.2 Client Configuration
**Scope**: Test client configuration scenarios
- **Test**: Transport failover
  - **Input**: Primary transport unavailable
  - **Expected**: Client attempts secondary transport
- **Test**: Feature hot-loading
  - **Input**: Add new features to running client
  - **Expected**: Client re-identifies with updated feature set

## Test Data Requirements

### Device Topologies
1. **Single Room**: 1 sensor, 1 actuator, 1 controller
2. **Multi-Room**: 3-5 rooms with 2-3 devices each
3. **Mixed Protocol**: TCP, WebSocket, and LoRa devices
4. **Large Scale**: 100+ simulated devices for performance testing

### Message Patterns
1. **Periodic**: Regular sensor readings (1-60 second intervals)
2. **Event-driven**: Motion detection, door opening, alarm triggers
3. **Control**: Light dimming, temperature adjustments, device on/off
4. **Batch**: Multiple commands or data points in sequence

### Error Conditions
1. **Network**: Timeouts, disconnections, partitions
2. **Protocol**: Malformed messages, invalid schemas
3. **Resource**: Memory limits, connection limits, queue overflow
4. **Logic**: Invalid commands, missing devices, circular references

## Success Criteria

### Functional Requirements
- All message types (command, query, data, status) work correctly
- Device registration and lifecycle management functions properly
- Pub/sub broker delivers messages reliably
- Web interface provides real-time device visibility and control
- Multiple transport protocols operate simultaneously

### Performance Requirements
- Support 100+ concurrent devices
- Handle 1000+ messages/second aggregate throughput
- Message latency <100ms under normal load
- Memory usage scales linearly with device count
- CPU usage <80% under maximum expected load

### Reliability Requirements
- Automatic reconnection after network failures
- No message loss during normal operations
- Graceful handling of malformed messages
- Server uptime >99.9% in stable network conditions
- Client sessions survive temporary network interruptions

## Test Environment Setup

### Infrastructure Requirements
- Test server with multiple network interfaces
- Network simulation tools for failure injection
- Load generation tools for performance testing
- Monitoring tools for resource usage measurement

### Mock Devices
- Temperature sensors with configurable reading patterns
- LED controllers with brightness and color control
- Motion detectors with occupancy simulation
- Thermostats with heating/cooling logic
- Smart outlets with power monitoring

### Test Automation
- Integration with Go testing framework
- CI/CD pipeline integration for automated testing
- Performance regression detection
- Test result reporting and archiving

This comprehensive test suite ensures the GoHab IoT system functions correctly across all supported scenarios, scales appropriately, and provides reliable home automation capabilities.
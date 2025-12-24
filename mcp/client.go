package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
	"github.com/mbocsi/gohab/services"
	"github.com/mbocsi/gohab/web"
)

// MCPClient acts as both an MCP server for external clients and a GoHab client
type MCPClient struct {
	// Core dependencies
	mcpServer *MCPServer
	services  *services.ServiceContainer
	addr      string

	// GoHab client metadata (embedded like WebClient)
	server.DeviceMetadata

	// Protocol translation
	correlationMap sync.Map // Track query/response correlations
	mcpContext     context.Context
	shutdown       context.CancelFunc
}

// NewMCPClient creates a new MCP client with the specified server (similar to NewWebClient)
func NewMCPClient(serviceContainer *services.ServiceContainer, mcpServer *MCPServer) *MCPClient {
	ctx, cancel := context.WithCancel(context.Background())

	client := &MCPClient{
		services:       serviceContainer,
		mcpServer:      mcpServer,
		mcpContext:     ctx,
		shutdown:       cancel,
		correlationMap: sync.Map{},
		DeviceMetadata: server.DeviceMetadata{
			Id:       "mcp-client",
			Name:     "MCP Interface Client",
			Firmware: "1.0.0",
			Features: make(map[string]proto.Feature),
			Subs:     make(map[string]struct{}),
		},
	}

	// Generate MCP features (what this client can do)
	client.Features = client.generateMCPFeatures()

	return client
}

// Start initializes the MCP server (similar to WebClient.Start)
func (m *MCPClient) Start() error {
	// Register MCP tools
	m.registerDeviceTools()
	m.registerFeatureTools()
	m.registerSystemTools()

	// Start the MCP server (the server handles transport details internally)
	return m.mcpServer.Start()
}

// Shutdown cleanly shuts down the MCP client (similar to WebClient.Shutdown)
func (m *MCPClient) Shutdown() error {
	slog.Info("Shutting down MCP client", "addr", m.addr)

	m.shutdown()

	if m.mcpServer != nil {
		return m.mcpServer.Shutdown()
	}
	return nil
}

// Meta returns device metadata (implements Client interface like WebClient)
func (m *MCPClient) Meta() *server.DeviceMetadata {
	return &m.DeviceMetadata
}

// Send handles incoming GoHab messages (implements Client interface like WebClient)
func (m *MCPClient) Send(msg proto.Message) error {
	slog.Debug("MCP client received message", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender)

	// Handle responses to queries we sent (correlation handling)
	if msg.Type == "response" && msg.CorrelationID != "" {
		if ctx, ok := m.correlationMap.Load(msg.CorrelationID); ok {
			if corrCtx, ok := ctx.(*CorrelationContext); ok {
				select {
				case corrCtx.ResponseCh <- msg:
				default:
					slog.Warn("Response channel full, dropping message", "correlation_id", msg.CorrelationID)
				}
			}
		}
	}

	return nil
}

// generateMCPFeatures declares what this client can do (similar to WebClient features)
func (m *MCPClient) generateMCPFeatures() map[string]proto.Feature {
	return map[string]proto.Feature{
		"mcp-tools": {
			Name:        "mcp-tools",
			Description: "Execute MCP tools via commands",
			Methods: proto.FeatureMethods{
				Command: proto.Method{
					InputSchema: map[string]proto.DataType{
						"tool_name": {Type: "string", Description: "Name of MCP tool to execute"},
						"arguments": {Type: "object", Description: "Tool arguments as JSON object"},
					},
					OutputSchema: map[string]proto.DataType{
						"result": {Type: "object", Description: "Tool execution result"},
						"error":  {Type: "string", Optional: true, Description: "Error message if failed"},
					},
				},
			},
		},
		"mcp-resources": {
			Name:        "mcp-resources",
			Description: "Access MCP resources via queries",
			Methods: proto.FeatureMethods{
				Query: proto.Method{
					InputSchema: map[string]proto.DataType{
						"resource_uri": {Type: "string", Description: "MCP resource URI to query"},
					},
					OutputSchema: map[string]proto.DataType{
						"content":  {Type: "object", Description: "Resource content"},
						"metadata": {Type: "object", Description: "Resource metadata"},
					},
				},
				Data: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"available_resources": {Type: "object", Description: "List of available MCP resources"},
					},
				},
			},
		},
		"mcp-notifications": {
			Name:        "mcp-notifications",
			Description: "Receive MCP system notifications",
			Methods: proto.FeatureMethods{
				Data: proto.Method{
					OutputSchema: map[string]proto.DataType{
						"notification_type": {Type: "string", Description: "Type of MCP notification"},
						"payload":           {Type: "object", Description: "Notification payload"},
					},
				},
			},
		},
	}
}

// CorrelationContext manages query/response correlations
type CorrelationContext struct {
	RequestID  string
	ResponseCh chan proto.Message
	Timeout    time.Duration
	CreatedAt  time.Time
}

// waitForResponse waits for a correlated response message
func (m *MCPClient) waitForResponse(correlationID string, timeout time.Duration) (proto.Message, error) {
	ctx := &CorrelationContext{
		RequestID:  correlationID,
		ResponseCh: make(chan proto.Message, 1),
		Timeout:    timeout,
		CreatedAt:  time.Now(),
	}

	m.correlationMap.Store(correlationID, ctx)
	defer m.correlationMap.Delete(correlationID)

	select {
	case response := <-ctx.ResponseCh:
		return response, nil
	case <-time.After(ctx.Timeout):
		return proto.Message{}, fmt.Errorf("timeout waiting for response")
	case <-m.mcpContext.Done():
		return proto.Message{}, fmt.Errorf("client shutting down")
	}
}

// sendMessage sends a message through the transport (similar to WebClient.SendMessage)
func (m *MCPClient) sendMessage(msg proto.Message) error {
	// The MCP client will be registered with a transport that handles message sending
	// This is similar to how WebClient sends messages through its transport
	if m.Transport != nil {
		// Use the full import path for the type assertion
		if transport, ok := m.Transport.(*web.InMemoryTransport); ok {
			return transport.SendMessage(msg)
		}
	}
	return fmt.Errorf("no transport configured or transport doesn't support direct message sending")
}

// registerDeviceTools registers MCP tools for device management
func (m *MCPClient) registerDeviceTools() {
	// List devices tool
	listDevicesTool := mcp.NewTool("list_devices",
		mcp.WithDescription("List all connected IoT devices"),
		mcp.WithBoolean("include_features",
			mcp.Description("Include device features/capabilities"),
		),
	)
	m.mcpServer.AddTool(listDevicesTool, m.handleListDevices)

	// Send command to topic tool
	sendCommandTool := mcp.NewTool("send_command_to_topic",
		mcp.WithDescription("Send command to the device that owns/provides a specific topic"),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Target topic/feature - command will be routed to the device that owns this topic"),
		),
		mcp.WithObject("payload",
			mcp.Required(),
			mcp.Description("Command payload"),
		),
	)
	m.mcpServer.AddTool(sendCommandTool, m.handleSendCommandToTopic)

	// Query topic tool
	queryTopicTool := mcp.NewTool("query_topic",
		mcp.WithDescription("Query the device that owns/provides a specific topic for current state"),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Target topic/feature - query will be routed to the device that owns this topic"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds"),
		),
	)
	m.mcpServer.AddTool(queryTopicTool, m.handleQueryTopic)
}

// registerFeatureTools registers MCP tools for feature management
func (m *MCPClient) registerFeatureTools() {
	// Broadcast to feature/topic tool
	broadcastTool := mcp.NewTool("broadcast_to_feature",
		mcp.WithDescription("Broadcast message to all devices subscribed to a feature/topic"),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Target topic/feature"),
		),
		mcp.WithString("message_type",
			mcp.Required(),
			mcp.Description("Type of message to broadcast"),
			mcp.Enum("data", "status", "command"),
		),
		mcp.WithObject("payload",
			mcp.Required(),
			mcp.Description("Message payload"),
		),
	)
	m.mcpServer.AddTool(broadcastTool, m.handleBroadcastToFeature)
}

// registerSystemTools registers MCP tools for system management
func (m *MCPClient) registerSystemTools() {
	// Get system status tool
	statusTool := mcp.NewTool("get_system_status",
		mcp.WithDescription("Get overall system health and statistics"),
		mcp.WithBoolean("include_transports",
			mcp.Description("Include transport information"),
		),
		mcp.WithBoolean("include_broker_stats",
			mcp.Description("Include broker statistics"),
		),
	)
	m.mcpServer.AddTool(statusTool, m.handleGetSystemStatus)
}

// Tool handler implementations - these use the services to get real data
func (m *MCPClient) handleListDevices(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeFeatures := request.GetBool("include_features", false)

	// Use the service container to get device data (same as WebClient does)
	devices, err := m.services.Device.ListDevices()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error listing devices: %v", err)), err
	}

	result := map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	}

	if includeFeatures {
		deviceFeatures := make(map[string]interface{})
		for _, device := range devices {
			features, err := m.services.Device.GetDeviceFeatures(device.ID)
			if err == nil {
				deviceFeatures[device.ID] = features
			}
		}
		result["features"] = deviceFeatures
	}

	resultBytes, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultBytes)), nil
}

func (m *MCPClient) handleSendCommandToTopic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic, err := request.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError("topic is required and must be a string"), err
	}

	// Get the payload as raw arguments and marshal it
	payloadArgs := request.GetRawArguments()
	if payloadMap, ok := payloadArgs.(map[string]interface{}); ok {
		if payload, exists := payloadMap["payload"]; exists {
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal payload: %v", err)), err
			}

			// Send command through our transport - server will route to topic source
			msg := proto.Message{
				Type:      "command",
				Topic:     topic,
				// No Recipient needed - server routes to topic source automatically
				Payload:   payloadBytes,
				Sender:    m.Id,
				Timestamp: time.Now().Unix(),
			}

			if err := m.sendMessage(msg); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to send command: %v", err)), err
			}

			return mcp.NewToolResultText(fmt.Sprintf("Command sent to topic %s (routed to topic source)", topic)), nil
		}
	}

	return mcp.NewToolResultError("payload is required"), fmt.Errorf("payload not provided")
}

func (m *MCPClient) handleQueryTopic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic, err := request.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError("topic is required and must be a string"), err
	}

	timeout := request.GetFloat("timeout", 30.0)
	correlationID := uuid.New().String()

	// Send query through our transport - server will route to topic source
	msg := proto.Message{
		Type:          "query",
		Topic:         topic,
		// No Recipient needed - server routes to topic source automatically
		CorrelationID: correlationID,
		Payload:       []byte(`{}`),
		Sender:        m.Id,
		Timestamp:     time.Now().Unix(),
	}

	if err := m.sendMessage(msg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send query: %v", err)), err
	}

	// Wait for response using correlation
	response, err := m.waitForResponse(correlationID, time.Duration(timeout)*time.Second)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query timeout or error: %v", err)), err
	}

	return mcp.NewToolResultText(fmt.Sprintf("Query response from topic %s: %s", topic, string(response.Payload))), nil
}

func (m *MCPClient) handleBroadcastToFeature(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic, err := request.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError("topic is required and must be a string"), err
	}

	messageType, err := request.RequireString("message_type")
	if err != nil {
		return mcp.NewToolResultError("message_type is required and must be a string"), err
	}

	// Get the payload as raw arguments and marshal it
	payloadArgs := request.GetRawArguments()
	if payloadMap, ok := payloadArgs.(map[string]interface{}); ok {
		if payload, exists := payloadMap["payload"]; exists {
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal payload: %v", err)), err
			}

			// Broadcast message through our transport (like WebClient does)
			msg := proto.Message{
				Type:      messageType,
				Topic:     topic,
				Payload:   payloadBytes,
				Sender:    m.Id,
				Timestamp: time.Now().Unix(),
			}

			if err := m.sendMessage(msg); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to broadcast message: %v", err)), err
			}

			return mcp.NewToolResultText(fmt.Sprintf("Broadcasted %s message to topic %s", messageType, topic)), nil
		}
	}

	return mcp.NewToolResultError("payload is required"), fmt.Errorf("payload not provided")
}

func (m *MCPClient) handleGetSystemStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeTransports := request.GetBool("include_transports", true)
	includeBrokerStats := request.GetBool("include_broker_stats", true)

	// Use the service container to get system data (same as WebClient does)
	status := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}

	// Get devices
	if devices, err := m.services.Device.ListDevices(); err == nil {
		status["devices"] = map[string]interface{}{
			"count": len(devices),
			"list":  devices,
		}
	}

	// Get features
	if features, err := m.services.Feature.ListFeatures(); err == nil {
		status["features"] = map[string]interface{}{
			"count": len(features),
			"list":  features,
		}
	}

	// Get transports if requested
	if includeTransports {
		if transports, err := m.services.Transport.ListTransports(); err == nil {
			status["transports"] = map[string]interface{}{
				"count": len(transports),
				"list":  transports,
			}
		}
	}

	// Get broker stats if requested (would need to add this to service layer)
	if includeBrokerStats {
		status["broker_stats"] = map[string]interface{}{
			"note": "Broker stats not yet implemented in service layer",
		}
	}

	resultBytes, _ := json.Marshal(status)
	return mcp.NewToolResultText(string(resultBytes)), nil
}

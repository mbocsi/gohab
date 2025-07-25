package services

import (
	"encoding/json"

	"github.com/mbocsi/gohab/server"
)

// marshalPayload marshals payload to JSON bytes
func marshalPayload(payload interface{}) ([]byte, error) {
	if payload == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
}

// convertDeviceMetadata converts server.DeviceMetadata to DeviceInfo
func convertDeviceMetadata(meta *server.DeviceMetadata) DeviceInfo {
	meta.Mu.RLock()
	defer meta.Mu.RUnlock()

	transportName := "Unknown"
	transportID := ""
	
	if meta.Transport != nil {
		transportMeta := meta.Transport.Meta()
		transportName = transportMeta.Name
		transportID = transportMeta.ID
	}

	return DeviceInfo{
		ID:            meta.Id,
		Name:          meta.Name,
		Firmware:      meta.Firmware,
		Features:  meta.Features,
		Subscriptions: meta.Subs,
		Connected:     true, // If it's in registry, it's connected
		TransportName: transportName,
		TransportID:   transportID,
	}
}

// convertTransportMeta converts transport metadata to TransportInfo
func convertTransportMeta(transport server.Transport) TransportInfo {
	meta := transport.Meta()
	status := "disconnected"
	if meta.Connected {
		status = "connected"
	}

	// Convert clients to DeviceInfo
	clients := make(map[string]DeviceInfo)
	for id, client := range meta.Clients {
		clients[id] = convertDeviceMetadata(client.Meta())
	}
	
	return TransportInfo{
		ID:          meta.ID,
		Name:        meta.Name,
		Type:        meta.Protocol,
		Address:     meta.Address,
		Description: meta.Description,
		Status:      status,
		Connected:   meta.Connected,
		MaxClients:  meta.MaxClients,
		Connections: len(meta.Clients),
		Clients:     clients,
	}
}

// validateMessageType validates message type
func validateMessageType(msgType string) error {
	validTypes := map[string]bool{
		"identify":    true,
		"command":     true,
		"query":       true,
		"response":    true,
		"data":        true,
		"status":      true,
		"subscribe":   true,
		"unsubscribe": true,
	}

	if !validTypes[msgType] {
		return ServiceError{
			Code:    ErrCodeInvalidInput,
			Message: "Invalid message type: " + msgType,
		}
	}

	return nil
}

// validateTopic validates topic name
func validateTopic(topic string) error {
	if topic == "" {
		return ServiceError{
			Code:    ErrCodeInvalidInput,
			Message: "Topic cannot be empty",
		}
	}

	// Add more validation rules as needed
	return nil
}

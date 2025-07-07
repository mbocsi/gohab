package services

import (
	"encoding/json"
	"time"

	"github.com/mbocsi/gohab/proto"
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
	
	return DeviceInfo{
		ID:            meta.Id,
		Name:          meta.Name,
		Firmware:      meta.Firmware,
		Capabilities:  meta.Capabilities,
		Subscriptions: meta.Subs,
		Connected:     true, // If it's in registry, it's connected
	}
}

// convertTransportMeta converts transport metadata to TransportInfo
func convertTransportMeta(index int, transport server.Transport) TransportInfo {
	meta := transport.Meta()
	status := "disconnected"
	if meta.Connected {
		status = "connected"
	}
	
	return TransportInfo{
		Index:       index,
		Type:        meta.Protocol,
		Status:      status,
		Connections: len(meta.Clients),
	}
}

// validateMessageType validates message type
func validateMessageType(msgType string) error {
	validTypes := map[string]bool{
		"identify":     true,
		"command":      true,
		"query":        true,
		"response":     true,
		"data":         true,
		"status":       true,
		"subscribe":    true,
		"unsubscribe":  true,
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

// createMessage creates a proto.Message from request
func createMessage(req MessageRequest, senderID string) (proto.Message, error) {
	if err := validateMessageType(req.Type); err != nil {
		return proto.Message{}, err
	}
	
	if err := validateTopic(req.Topic); err != nil {
		return proto.Message{}, err
	}
	
	payloadBytes, err := marshalPayload(req.Payload)
	if err != nil {
		return proto.Message{}, ServiceError{
			Code:    ErrCodeInvalidInput,
			Message: "Failed to marshal payload",
			Cause:   err,
		}
	}
	
	return proto.Message{
		Type:      req.Type,
		Topic:     req.Topic,
		Payload:   payloadBytes,
		Sender:    senderID,
		Recipient: req.Recipient,
		Timestamp: time.Now().Unix(),
	}, nil
}
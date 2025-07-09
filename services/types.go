package services

import (
	"github.com/mbocsi/gohab/proto"
)

// DeviceInfo represents device information for the service layer
type DeviceInfo struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Firmware      string                       `json:"firmware"`
	Capabilities  map[string]proto.Capability `json:"capabilities"`
	Subscriptions map[string]struct{}         `json:"subscriptions"`
	Connected     bool                         `json:"connected"`
	TransportName string                       `json:"transport_name"`
	TransportID   string                       `json:"transport_id"`
}

// FeatureInfo represents feature/capability information
type FeatureInfo struct {
	Topic       string            `json:"topic"`
	Capability  proto.Capability  `json:"capability"`
	SourceID    string            `json:"source_id"`
	SourceName  string            `json:"source_name"`
	Subscribers []DeviceInfo      `json:"subscribers"`
}

// TransportInfo represents transport connection information
type TransportInfo struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        string              `json:"type"`
	Address     string              `json:"address"`
	Description string              `json:"description"`
	Status      string              `json:"status"`
	Connected   bool                `json:"connected"`
	MaxClients  int                 `json:"max_clients"`
	Connections int                 `json:"connections"`
	Clients     map[string]DeviceInfo `json:"clients"`
}


// ServiceError represents structured service layer errors
type ServiceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"cause,omitempty"`
}

func (e ServiceError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Common error codes
const (
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeInvalidInput  = "INVALID_INPUT"
	ErrCodeTimeout       = "TIMEOUT"
	ErrCodeInternal      = "INTERNAL_ERROR"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
)
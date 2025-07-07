package services

import (
	"time"

	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// DeviceService handles device-related operations
type DeviceService interface {
	// Device management
	ListDevices() ([]DeviceInfo, error)
	GetDevice(id string) (*DeviceInfo, error)
	RenameDevice(id, name string) error
	GetDeviceCapabilities(id string) (map[string]proto.Capability, error)
	
	// Device status
	IsDeviceConnected(id string) (bool, error)
	GetDeviceSubscriptions(id string) ([]string, error)
}

// FeatureService handles feature/capability operations
type FeatureService interface {
	// Feature discovery
	ListFeatures() (map[string]FeatureInfo, error)
	GetFeature(topic string) (*FeatureInfo, error)
	GetFeatureSubscribers(topic string) ([]DeviceInfo, error)
	
	// Feature management
	GetFeaturesForDevice(deviceID string) ([]FeatureInfo, error)
}

// MessagingService handles message routing and correlation
type MessagingService interface {
	// Synchronous messaging (with response correlation)
	SendQuery(topic string, payload interface{}, timeout ...time.Duration) (*QueryResponse, error)
	
	// Asynchronous messaging
	SendCommand(topic string, payload interface{}) error
	SendData(topic string, payload interface{}) error
	SendStatus(topic string, payload interface{}) error
	
	// Generic message sending
	SendMessage(req MessageRequest) error
	
	// Subscription management
	Subscribe(topic string, client server.Client) error
	Unsubscribe(topic string, client server.Client) error
}

// TransportService handles transport information
type TransportService interface {
	ListTransports() ([]TransportInfo, error)
	GetTransport(index int) (*TransportInfo, error)
	GetTransportStats() (map[string]interface{}, error)
}

// ServiceContainer holds all service implementations
type ServiceContainer struct {
	Device    DeviceService
	Feature   FeatureService
	Messaging MessagingService
	Transport TransportService
}
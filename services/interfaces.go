package services

import (
	"github.com/mbocsi/gohab/proto"
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

// TransportService handles transport information
type TransportService interface {
	ListTransports() ([]TransportInfo, error)
	GetTransport(id string) (*TransportInfo, error)
	GetTransportStats() (map[string]interface{}, error)
}

// ServiceContainer holds all service implementations
type ServiceContainer struct {
	Device    DeviceService
	Feature   FeatureService
	Transport TransportService
}

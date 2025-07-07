package services

import (
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// DeviceServiceImpl implements DeviceService
type DeviceServiceImpl struct {
	registry *server.DeviceRegistry
}

// NewDeviceService creates a new device service
func NewDeviceService(registry *server.DeviceRegistry) DeviceService {
	return &DeviceServiceImpl{
		registry: registry,
	}
}

// ListDevices returns all registered devices
func (ds *DeviceServiceImpl) ListDevices() ([]DeviceInfo, error) {
	devices := ds.registry.List()
	result := make([]DeviceInfo, 0, len(devices))
	
	for _, device := range devices {
		result = append(result, convertDeviceMetadata(device.Meta()))
	}
	
	return result, nil
}

// GetDevice returns a specific device by ID
func (ds *DeviceServiceImpl) GetDevice(id string) (*DeviceInfo, error) {
	device, exists := ds.registry.Get(id)
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Device not found: " + id,
		}
	}
	
	info := convertDeviceMetadata(device.Meta())
	return &info, nil
}

// RenameDevice renames a device
func (ds *DeviceServiceImpl) RenameDevice(id, name string) error {
	device, exists := ds.registry.Get(id)
	if !exists {
		return ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Device not found: " + id,
		}
	}
	
	if name == "" {
		return ServiceError{
			Code:    ErrCodeInvalidInput,
			Message: "Device name cannot be empty",
		}
	}
	
	meta := device.Meta()
	meta.Mu.Lock()
	meta.Name = name
	meta.Mu.Unlock()
	
	return nil
}

// GetDeviceCapabilities returns device capabilities
func (ds *DeviceServiceImpl) GetDeviceCapabilities(id string) (map[string]proto.Capability, error) {
	device, exists := ds.registry.Get(id)
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Device not found: " + id,
		}
	}
	
	meta := device.Meta()
	meta.Mu.RLock()
	capabilities := make(map[string]proto.Capability)
	for k, v := range meta.Capabilities {
		capabilities[k] = v
	}
	meta.Mu.RUnlock()
	
	return capabilities, nil
}

// IsDeviceConnected checks if device is connected
func (ds *DeviceServiceImpl) IsDeviceConnected(id string) (bool, error) {
	_, exists := ds.registry.Get(id)
	return exists, nil
}

// GetDeviceSubscriptions returns device subscriptions
func (ds *DeviceServiceImpl) GetDeviceSubscriptions(id string) ([]string, error) {
	device, exists := ds.registry.Get(id)
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Device not found: " + id,
		}
	}
	
	meta := device.Meta()
	meta.Mu.RLock()
	subs := make([]string, 0, len(meta.Subs))
	for topic := range meta.Subs {
		subs = append(subs, topic)
	}
	meta.Mu.RUnlock()
	
	return subs, nil
}
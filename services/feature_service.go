package services

import (
	"github.com/mbocsi/gohab/server"
)

// FeatureServiceImpl implements FeatureService
type FeatureServiceImpl struct {
	registry     *server.DeviceRegistry
	broker       *server.Broker
	topicSources map[string]string
}

// NewFeatureService creates a new feature service
func NewFeatureService(registry *server.DeviceRegistry, broker *server.Broker, topicSources map[string]string) FeatureService {
	return &FeatureServiceImpl{
		registry:     registry,
		broker:       broker,
		topicSources: topicSources,
	}
}

// ListFeatures returns all available features
func (fs *FeatureServiceImpl) ListFeatures() (map[string]FeatureInfo, error) {
	features := make(map[string]FeatureInfo)
	
	for topic := range fs.topicSources {
		feature, err := fs.GetFeature(topic)
		if err != nil {
			// Skip features that can't be retrieved
			continue
		}
		features[topic] = *feature
	}
	
	return features, nil
}

// GetFeature returns a specific feature by topic
func (fs *FeatureServiceImpl) GetFeature(topic string) (*FeatureInfo, error) {
	sourceID, exists := fs.topicSources[topic]
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Feature not found: " + topic,
		}
	}
	
	device, exists := fs.registry.Get(sourceID)
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Feature source device not found: " + sourceID,
		}
	}
	
	meta := device.Meta()
	meta.Mu.RLock()
	capability, exists := meta.Capabilities[topic]
	if !exists {
		meta.Mu.RUnlock()
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Capability not found for topic: " + topic,
		}
	}
	sourceName := meta.Name
	meta.Mu.RUnlock()
	
	// Get subscribers
	subscribers, err := fs.GetFeatureSubscribers(topic)
	if err != nil {
		// Continue without subscribers if error occurs
		subscribers = []DeviceInfo{}
	}
	
	return &FeatureInfo{
		Topic:       topic,
		Capability:  capability,
		SourceID:    sourceID,
		SourceName:  sourceName,
		Subscribers: subscribers,
	}, nil
}

// GetFeatureSubscribers returns devices subscribed to a feature
func (fs *FeatureServiceImpl) GetFeatureSubscribers(topic string) ([]DeviceInfo, error) {
	subscribers := fs.broker.Subs(topic)
	result := make([]DeviceInfo, 0, len(subscribers))
	
	for client := range subscribers {
		result = append(result, convertDeviceMetadata(client.Meta()))
	}
	
	return result, nil
}

// GetFeaturesForDevice returns features provided by a specific device
func (fs *FeatureServiceImpl) GetFeaturesForDevice(deviceID string) ([]FeatureInfo, error) {
	device, exists := fs.registry.Get(deviceID)
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Device not found: " + deviceID,
		}
	}
	
	meta := device.Meta()
	meta.Mu.RLock()
	capabilities := meta.Capabilities
	meta.Mu.RUnlock()
	
	var features []FeatureInfo
	for topic, capability := range capabilities {
		// Get subscribers for this feature
		subscribers, err := fs.GetFeatureSubscribers(topic)
		if err != nil {
			subscribers = []DeviceInfo{}
		}
		
		features = append(features, FeatureInfo{
			Topic:       topic,
			Capability:  capability,
			SourceID:    deviceID,
			SourceName:  meta.Name,
			Subscribers: subscribers,
		})
	}
	
	return features, nil
}
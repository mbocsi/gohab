package services

import (
	"github.com/mbocsi/gohab/server"
)

// FeatureServiceImpl implements FeatureService
type FeatureServiceImpl struct {
	registry        *server.DeviceRegistry
	broker          *server.Broker
	getTopicSources func() map[string]string
}

// NewFeatureService creates a new feature service
func NewFeatureService(registry *server.DeviceRegistry, broker *server.Broker, topicSources func() map[string]string) FeatureService {
	return &FeatureServiceImpl{
		registry:        registry,
		broker:          broker,
		getTopicSources: topicSources,
	}
}

// ListFeatures returns all available features
func (fs *FeatureServiceImpl) ListFeatures() (map[string]FeatureInfo, error) {
	features := make(map[string]FeatureInfo)

	for topic := range fs.getTopicSources() {
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
	sourceID, exists := fs.getTopicSources()[topic]
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
	feature, exists := meta.Features[topic]
	if !exists {
		meta.Mu.RUnlock()
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Feature not found for topic: " + topic,
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
		Feature:  feature,
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
	features := meta.Features
	meta.Mu.RUnlock()

	var featuresList []FeatureInfo
	for topic, feature := range features {
		// Get subscribers for this feature
		subscribers, err := fs.GetFeatureSubscribers(topic)
		if err != nil {
			subscribers = []DeviceInfo{}
		}

		featuresList = append(featuresList, FeatureInfo{
			Topic:       topic,
			Feature:  feature,
			SourceID:    deviceID,
			SourceName:  meta.Name,
			Subscribers: subscribers,
		})
	}

	return featuresList, nil
}

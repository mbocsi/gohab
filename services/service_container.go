package services

import (
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// ServiceManagerImpl manages all services with dependency injection
type ServiceManagerImpl struct {
	registry     *server.DeviceRegistry
	broker       *server.Broker
	transports   []server.Transport
	topicSources map[string]string
	handleFunc   func(proto.Message)

	services *ServiceContainer
}

// NewServiceManager creates a new service manager
func NewServiceManager(
	registry *server.DeviceRegistry,
	broker *server.Broker,
	transports []server.Transport,
	topicSources map[string]string,
	handleFunc func(proto.Message),
) *ServiceManagerImpl {

	sm := &ServiceManagerImpl{
		registry:     registry,
		broker:       broker,
		transports:   transports,
		topicSources: topicSources,
		handleFunc:   handleFunc,
	}

	// Initialize services
	sm.services = &ServiceContainer{
		Device:    NewDeviceService(registry),
		Feature:   NewFeatureService(registry, broker, topicSources),
		Transport: NewTransportService(transports),
	}

	return sm
}

// GetServices returns the service container
func (sm *ServiceManagerImpl) GetServices() *ServiceContainer {
	return sm.services
}

// HandleMessage handles incoming messages and routes them to appropriate services
func (sm *ServiceManagerImpl) HandleMessage(msg proto.Message) {
	// Continue with normal message handling
	sm.handleFunc(msg)
}

// // UpdateTopicSources updates the topic sources map (thread-safe)
// func (sm *ServiceManagerImpl) UpdateTopicSources(newSources map[string]string) {
// 	// Clear existing sources
// 	for k := range sm.topicSources {
// 		delete(sm.topicSources, k)
// 	}

// 	// Add new sources
// 	for k, v := range newSources {
// 		sm.topicSources[k] = v
// 	}
// }

// // GetTopicSources returns a copy of topic sources (thread-safe)
// func (sm *ServiceManagerImpl) GetTopicSources() map[string]string {
// 	sources := make(map[string]string)
// 	for k, v := range sm.topicSources {
// 		sources[k] = v
// 	}

// 	return sources
// }

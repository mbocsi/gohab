package web

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
	"github.com/mbocsi/gohab/services"
)

// WebServer interface defines what we need from the main server
type WebServer interface {
	GetBroker() *server.Broker
	GetRegistry() *server.DeviceRegistry
	GetTransports() []server.Transport
	GetTopicSources() map[string]string
	Handle(msg proto.Message)
}

// WebClient acts as both a web UI server and a client with pub/sub capabilities
type WebClient struct {
	server    WebServer
	services  *services.ServiceContainer
	templates *Templates
	server.DeviceMetadata
}

// NewWebClient creates a new web client that can serve the UI and act as a client
func NewWebClient(srv WebServer, serviceContainer *services.ServiceContainer) *WebClient {
	return &WebClient{
		server:    srv,
		services:  serviceContainer,
		templates: NewTemplates("templates/layout/*.html"),
		DeviceMetadata: server.DeviceMetadata{
			Id:           "web-ui",
			Name:         "Web UI Client",
			Firmware:     "1.0.0",
			Capabilities: make(map[string]proto.Capability),
			Subs:         make(map[string]struct{}),
		},
	}
}

// Meta returns the client metadata (implements Client interface)
func (w *WebClient) Meta() *server.DeviceMetadata {
	return &w.DeviceMetadata
}

// Send sends a message (implements Client interface)
func (w *WebClient) Send(msg proto.Message) error {
	// For web client, we can handle incoming messages here
	// This could be used for real-time updates to the web UI
	slog.Info("WebClient received message", "type", msg.Type, "topic", msg.Topic)
	return nil
}

// Subscribe to a topic for real-time updates
func (w *WebClient) Subscribe(topic string) error {
	return w.services.Messaging.Subscribe(topic, w)
}

// Unsubscribe from a topic
func (w *WebClient) Unsubscribe(topic string) error {
	return w.services.Messaging.Unsubscribe(topic, w)
}

// SendMessage sends a message through the broker
func (w *WebClient) SendMessage(msgType, topic string, payload interface{}) error {
	return w.services.Messaging.SendMessage(services.MessageRequest{
		Type:    msgType,
		Topic:   topic,
		Payload: payload,
	})
}

// RenameDevice is an elevated function that renames a device
func (w *WebClient) RenameDevice(deviceID, newName string) error {
	return w.services.Device.RenameDevice(deviceID, newName)
}

// Routes returns the HTTP routes for the web UI
func (w *WebClient) Routes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	r.Get("/", w.HandleHome)
	r.Get("/devices", w.HandleDevices)
	r.Get("/devices/{id}", w.HandleDeviceDetail)
	r.Get("/features", w.HandleFeatures)
	r.Get("/features/{name}", w.HandleFeatureDetail)
	r.Get("/transports", w.HandleTransports)
	r.Get("/transports/{i}", w.HandleTransportDetail)
	r.Post("/api/devices/{id}/rename", w.HandleDeviceRename)
	r.Post("/api/messages", w.HandleSendMessage)
	r.Post("/api/queries", w.HandleSendQuery)
	return r
}

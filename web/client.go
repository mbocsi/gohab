package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
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
	templates *Templates
	server.DeviceMetadata
}

// NewWebClient creates a new web client that can serve the UI and act as a client
func NewWebClient(srv WebServer) *WebClient {
	return &WebClient{
		server:    srv,
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

	w.server.GetBroker().Subscribe(topic, w)
	w.Subs[topic] = struct{}{}
	w.DeviceMetadata.Subs[topic] = struct{}{}
	return nil
}

// Unsubscribe from a topic
func (w *WebClient) Unsubscribe(topic string) error {

	w.server.GetBroker().Unsubscribe(topic, w)
	delete(w.Subs, topic)
	delete(w.DeviceMetadata.Subs, topic)
	return nil
}

// SendMessage sends a message through the broker
func (w *WebClient) SendMessage(msgType, topic string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := proto.Message{
		Type:      msgType,
		Topic:     topic,
		Payload:   payloadBytes,
		Sender:    w.Id,
		Timestamp: time.Now().Unix(),
	}

	w.server.Handle(msg)
	return nil
}

// RenameDevice is an elevated function that renames a device
func (w *WebClient) RenameDevice(deviceID, newName string) error {
	client, ok := w.server.GetRegistry().Get(deviceID)
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	client.Meta().Mu.Lock()
	client.Meta().Name = newName
	client.Meta().Mu.Unlock()

	slog.Info("Device renamed", "id", deviceID, "newName", newName)
	return nil
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
	return r
}

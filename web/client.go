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
	"github.com/mbocsi/gohab/services"
)

// WebClient acts as both a web UI server and a client with pub/sub capabilities
type WebClient struct {
	addr      string
	server    *http.Server
	services  *services.ServiceContainer
	transport *WebTransport
	templates *Templates
	server.DeviceMetadata
}

func NewWebClient(serviceContainer *services.ServiceContainer) *WebClient {
	client := &WebClient{
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

	return client
}

func (w *WebClient) Start(addr string) error {
	w.server = &http.Server{
		Addr:    addr,
		Handler: w.Routes(),
	}
	w.addr = addr
	slog.Info("Starting http server", "addr", addr)
	return w.server.ListenAndServe()
}

func (w *WebClient) Shutdown() error {
	slog.Info("Shutting down http server", "addr", w.addr)
	if w.server != nil {
		return w.server.Close()
	}
	return nil
}

func (w *WebClient) Meta() *server.DeviceMetadata {
	w.DeviceMetadata.Mu.RLock()
	defer w.DeviceMetadata.Mu.RUnlock()
	return &w.DeviceMetadata
}

func (w *WebClient) Send(msg proto.Message) error {
	// For web client, we can handle incoming messages here
	// This could be used for real-time updates to the web UI
	slog.Info("WebClient received message", "type", msg.Type, "topic", msg.Topic)
	return nil
}

func (w *WebClient) Subscribe(topic string) error {
	subscribeMsg := proto.Message{
		Type:      "subscribe",
		Payload:   []byte(`{"topics":["` + topic + `"]}`),
		Sender:    w.Id,
		Timestamp: time.Now().Unix(),
	}

	if err := w.transport.SendMessage(subscribeMsg); err != nil {
		return err
	}

	w.Mu.Lock()
	w.Subs[topic] = struct{}{}
	w.Mu.Unlock()
	return nil
}

func (w *WebClient) Unsubscribe(topic string) error {
	unsubscribeMsg := proto.Message{
		Type:      "unsubscribe",
		Payload:   []byte(`{"topics":["` + topic + `"]}`),
		Sender:    w.Id,
		Timestamp: time.Now().Unix(),
	}

	if err := w.transport.SendMessage(unsubscribeMsg); err != nil {
		return err
	}

	w.Mu.Lock()
	delete(w.Subs, topic)
	w.Mu.Unlock()
	return nil
}

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

	return w.transport.SendMessage(msg)
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
	return r
}

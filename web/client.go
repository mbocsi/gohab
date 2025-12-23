package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
	"github.com/mbocsi/gohab/services"
)

// SSEConnection represents an active SSE connection for a topic
type SSEConnection struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	topic   string
	done    chan struct{}
}

// WebClient acts as both a web UI server and a client with pub/sub capabilities
type WebClient struct {
	addr      string
	server    *http.Server
	services  *services.ServiceContainer
	templates *Templates
	server.DeviceMetadata

	// SSE connections management
	sseConnections map[string][]*SSEConnection
	sseMutex       sync.RWMutex
}

func NewWebClient(serviceContainer *services.ServiceContainer) *WebClient {
	client := &WebClient{
		services:       serviceContainer,
		templates:      NewTemplates(filepath.Join(getProjectRoot(), "templates/layout/*.html")),
		sseConnections: make(map[string][]*SSEConnection),
		DeviceMetadata: server.DeviceMetadata{
			Id:       "web-ui",
			Name:     "Web UI Client",
			Firmware: "1.0.0",
			Features: make(map[string]proto.Feature),
			Subs:     make(map[string]struct{}),
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
	return &w.DeviceMetadata
}

func (w *WebClient) Send(msg proto.Message) error {
	w.broadcastToSSE(msg)

	slog.Debug("WebClient received message", "type", msg.Type, "topic", msg.Topic)
	return nil
}

func (w *WebClient) Subscribe(topic string) error {
	subscribeMsg := proto.Message{
		Type:      "subscribe",
		Payload:   []byte(`{"topics":["` + topic + `"]}`),
		Sender:    w.Id,
		Timestamp: time.Now().Unix(),
	}

	if w.Transport != nil {
		if transport, ok := w.Transport.(*InMemoryTransport); ok {
			if err := transport.SendMessage(subscribeMsg); err != nil {
				slog.Error("Failed to send subscribe message", "topic", topic, "error", err)
				return err
			}
		} else {
			slog.Warn("Transport is not InMemoryTransport", "topic", topic)
		}
	} else {
		slog.Warn("WebClient has no transport", "topic", topic)
	}

	return nil
}

func (w *WebClient) Unsubscribe(topic string) error {
	unsubscribeMsg := proto.Message{
		Type:      "unsubscribe",
		Payload:   []byte(`{"topics":["` + topic + `"]}`),
		Sender:    w.Id,
		Timestamp: time.Now().Unix(),
	}

	if w.Transport != nil {
		if transport, ok := w.Transport.(*InMemoryTransport); ok {
			if err := transport.SendMessage(unsubscribeMsg); err != nil {
				return err
			}
		}
	}

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

	if w.Transport != nil {
		if transport, ok := w.Transport.(*InMemoryTransport); ok {
			return transport.SendMessage(msg)
		}
	}
	return nil
}

// broadcastToSSE sends a message to all SSE connections for a topic
func (w *WebClient) broadcastToSSE(msg proto.Message) {
	w.sseMutex.RLock()
	connections := w.sseConnections[msg.Topic]
	w.sseMutex.RUnlock()

	if len(connections) == 0 {
		return
	}

	// Create template data with formatted time
	templateData := struct {
		Type          string
		Topic         string
		Payload       string
		Sender        string
		Timestamp     int64
		FormattedTime string
	}{
		Type:          msg.Type,
		Topic:         msg.Topic,
		Payload:       string(msg.Payload),
		Sender:        msg.Sender,
		Timestamp:     msg.Timestamp,
		FormattedTime: time.Unix(msg.Timestamp, 0).Format("15:04:05"),
	}

	// Render the template
	var buf strings.Builder
	if err := w.templates.pages["features"].Render(&buf, "live-message", templateData); err != nil {
		slog.Error("Failed to render SSE message template", "error", err)
		return
	}

	// Remove newlines from template output for SSE
	cleanHTML := strings.ReplaceAll(buf.String(), "\n", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\r", "")

	// Send to all connections for this topic
	for _, conn := range connections {
		select {
		case <-conn.done:
			// Connection is closed, skip
			continue
		default:
			fmt.Fprintf(conn.writer, "event: message\n")
			fmt.Fprintf(conn.writer, "data: %s\n\n", cleanHTML)
			conn.flusher.Flush()
		}
	}
}

// addSSEConnection adds a new SSE connection for a topic
func (w *WebClient) addSSEConnection(topic string, conn *SSEConnection) {
	w.sseMutex.Lock()
	defer w.sseMutex.Unlock()

	w.sseConnections[topic] = append(w.sseConnections[topic], conn)
}

// removeSSEConnection removes an SSE connection for a topic
func (w *WebClient) removeSSEConnection(topic string, conn *SSEConnection) {
	w.sseMutex.Lock()
	defer w.sseMutex.Unlock()

	connections := w.sseConnections[topic]
	for i, c := range connections {
		if c == conn {
			// Remove this connection
			w.sseConnections[topic] = append(connections[:i], connections[i+1:]...)
			break
		}
	}

	// If no more connections for this topic, unsubscribe
	if len(w.sseConnections[topic]) == 0 {
		delete(w.sseConnections, topic)
		w.Unsubscribe(topic)
	}
}

// RenameDevice is an elevated function that renames a device
func (w *WebClient) RenameDevice(deviceID, newName string) error {
	return w.services.Device.RenameDevice(deviceID, newName)
}

// Routes returns the HTTP routes for the web UI
func (w *WebClient) Routes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(getProjectRoot(), "assets")))))
	r.Get("/", w.HandleHome)
	r.Get("/devices", w.HandleDevices)
	r.Get("/devices/{id}", w.HandleDeviceDetail)
	r.Get("/devices/{id}/rename-form", w.HandleDeviceRenameForm)
	r.Get("/devices/{id}/cancel-rename", w.HandleDeviceRenameCancel)
	r.Get("/features", w.HandleFeatures)
	r.Get("/features/{name}", w.HandleFeatureDetail)
	r.Get("/transports", w.HandleTransports)
	r.Get("/transports/{id}", w.HandleTransportDetail)
	r.Get("/events/topics/{name}", w.HandleTopicEvents)
	r.Post("/api/devices/{id}/rename", w.HandleDeviceRename)
	r.Post("/api/messages", w.HandleSendMessage)
	return r
}

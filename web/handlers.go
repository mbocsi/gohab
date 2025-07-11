package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/services"
)

func (w *WebClient) HandleHome(wr http.ResponseWriter, r *http.Request) {
	http.Redirect(wr, r, "/devices", http.StatusMovedPermanently)
}

func (w *WebClient) HandleDevices(wr http.ResponseWriter, r *http.Request) {
	devices, err := w.services.Device.ListDevices()
	if err != nil {
		w.handleError(wr, err)
		return
	}

	w.templates.pages["devices"].RenderPage(wr, map[string]interface{}{
		"Devices": devices,
	})
}

func (w *WebClient) HandleDeviceDetail(wr http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, err := w.services.Device.GetDevice(id)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		devices, err := w.services.Device.ListDevices()
		if err != nil {
			w.handleError(wr, err)
			return
		}
		w.templates.pages["devices"].RenderPage(wr, map[string]interface{}{
			"Device":  device,
			"Devices": devices,
		})
	} else {
		wr.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := w.templates.pages["devices"].Render(wr, "content", map[string]interface{}{
			"Device": device,
		}); err != nil {
			w.handleError(wr, err)
			return
		}
	}
}

func (w *WebClient) HandleFeatures(wr http.ResponseWriter, r *http.Request) {
	features, err := w.services.Feature.ListFeatures()
	if err != nil {
		w.handleError(wr, err)
		return
	}

	w.templates.pages["features"].RenderPage(wr, map[string]any{
		"Features": features,
	})
}

func (w *WebClient) HandleFeatureDetail(wr http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "name")
	feature, err := w.services.Feature.GetFeature(topic)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		features, err := w.services.Feature.ListFeatures()
		if err != nil {
			w.handleError(wr, err)
			return
		}

		w.templates.pages["features"].RenderPage(wr, map[string]interface{}{
			"Feature":       feature.Feature,
			"FeatureSource": feature,
			"Features":      features,
			"Subscriptions": feature.Subscribers,
		})
	} else {
		wr.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := w.templates.pages["features"].Render(wr, "content", map[string]interface{}{
			"Feature":       feature.Feature,
			"FeatureSource": feature,
			"Subscriptions": feature.Subscribers,
		}); err != nil {
			w.handleError(wr, err)
			return
		}
	}
}

func (w *WebClient) HandleTransports(wr http.ResponseWriter, r *http.Request) {
	transports, err := w.services.Transport.ListTransports()
	if err != nil {
		w.handleError(wr, err)
		return
	}

	w.templates.pages["transports"].RenderPage(wr, map[string]interface{}{
		"Transports": transports,
	})
}

func (w *WebClient) HandleTransportDetail(wr http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	transport, err := w.services.Transport.GetTransport(id)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		transports, err := w.services.Transport.ListTransports()
		if err != nil {
			w.handleError(wr, err)
			return
		}

		w.templates.pages["transports"].RenderPage(wr, map[string]interface{}{
			"Transports": transports,
			"Transport":  transport,
		})
	} else {
		wr.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := w.templates.pages["transports"].Render(wr, "content", map[string]interface{}{
			"Transport": transport,
		}); err != nil {
			w.handleError(wr, err)
			return
		}
	}
}

// HandleDeviceRename handles device renaming from web forms
func (w *WebClient) HandleDeviceRename(wr http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	if err := r.ParseForm(); err != nil {
		http.Error(wr, "Invalid form data", http.StatusBadRequest)
		return
	}

	newName := r.FormValue("name")
	if err := w.RenameDevice(deviceID, newName); err != nil {
		http.Error(wr, err.Error(), http.StatusBadRequest)
		return
	}

	// Return just the new name for HTMX to update the device name span
	wr.Header().Set("Content-Type", "text/plain")
	wr.Header().Set("HX-Trigger", "clearRenameForm")
	fmt.Fprint(wr, newName)
}

// HandleDeviceRenameForm shows the rename form
func (w *WebClient) HandleDeviceRenameForm(wr http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, err := w.services.Device.GetDevice(id)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := w.templates.pages["devices"].Render(wr, "rename-form", map[string]interface{}{
		"Device": device,
	}); err != nil {
		w.handleError(wr, err)
		return
	}
}

// HandleDeviceRenameCancel clears the rename form
func (w *WebClient) HandleDeviceRenameCancel(wr http.ResponseWriter, r *http.Request) {
	// Return empty content to clear the rename container
	wr.WriteHeader(http.StatusOK)
}

// HandleSendMessage allows the web UI to send messages like a client
func (w *WebClient) HandleSendMessage(wr http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string      `json:"type"`
		Topic   string      `json:"topic"`
		Payload interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.handleError(wr, err)
		return
	}

	if err := w.SendMessage(req.Type, req.Topic, req.Payload); err != nil {
		w.handleError(wr, err)
		return
	}

	wr.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(wr, "Message sent to %s/%s", req.Topic, req.Type)
}

// HandleTopicEvents streams Server-Sent Events for a specific topic
func (w *WebClient) HandleTopicEvents(wr http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "name")

	// Set SSE headers
	wr.Header().Set("Content-Type", "text/event-stream")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Connection", "keep-alive")
	wr.Header().Set("Access-Control-Allow-Origin", "*")
	wr.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Check if we can flush
	flusher, ok := wr.(http.Flusher)
	if !ok {
		slog.Error("Streaming unsupported", "topic", topic)
		http.Error(wr, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create SSE connection
	conn := &SSEConnection{
		writer:  wr,
		flusher: flusher,
		topic:   topic,
		done:    make(chan struct{}),
	}

	// Add connection and subscribe to topic if this is the first connection
	w.sseMutex.RLock()
	isFirstConnection := len(w.sseConnections[topic]) == 0
	w.sseMutex.RUnlock()

	w.addSSEConnection(topic, conn)

	if isFirstConnection {
		w.Subscribe(topic)
	}

	// Clean up when the connection closes
	defer func() {
		close(conn.done)
		w.removeSSEConnection(topic, conn)
	}()

	// Send initial connection message as HTML template
	connectionData := struct {
		Type          string
		Topic         string
		Payload       string
		Sender        string
		Timestamp     int64
		FormattedTime string
	}{
		Type:          "connected",
		Topic:         topic,
		Payload:       "Connected to live message stream",
		Sender:        "system",
		Timestamp:     time.Now().Unix(),
		FormattedTime: time.Now().Format("15:04:05"),
	}

	var buf strings.Builder
	if err := w.templates.pages["features"].Render(&buf, "live-message", connectionData); err != nil {
		slog.Error("Failed to render connection message template", "error", err)
		fmt.Fprintf(wr, "event: message\n")
		fmt.Fprintf(wr, "data: <p>Connected to %s</p>\n\n", topic)
	} else {
		// Remove newlines from template output for SSE
		cleanHTML := strings.ReplaceAll(buf.String(), "\n", "")
		cleanHTML = strings.ReplaceAll(cleanHTML, "\r", "")
		fmt.Fprintf(wr, "event: message\n")
		fmt.Fprintf(wr, "data: %s\n\n", cleanHTML)
	}
	flusher.Flush()

	// Keep connection alive until client disconnects
	<-r.Context().Done()
}

// handleError handles service errors with proper HTTP status codes
func (w *WebClient) handleError(wr http.ResponseWriter, err error) {
	slog.Error("Service error", "error", err)

	if serviceErr, ok := err.(services.ServiceError); ok {
		status := http.StatusInternalServerError
		switch serviceErr.Code {
		case services.ErrCodeNotFound:
			status = http.StatusNotFound
		case services.ErrCodeInvalidInput:
			status = http.StatusBadRequest
		case services.ErrCodeTimeout:
			status = http.StatusRequestTimeout
		case services.ErrCodeUnauthorized:
			status = http.StatusUnauthorized
		}

		http.Error(wr, serviceErr.Message, status)
		return
	}

	http.Error(wr, "Internal server error", http.StatusInternalServerError)
}

// handleTemplateError handles template errors
func (w *WebClient) handleTemplateError(wr http.ResponseWriter, err error, page string) {
	slog.Error("Template error", "error", err, "page", page)
	http.Error(wr, "Template error: "+err.Error(), http.StatusInternalServerError)
}

package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/server"
)

func (w *WebClient) HandleHome(wr http.ResponseWriter, r *http.Request) {
	http.Redirect(wr, r, "/devices", http.StatusMovedPermanently)
}

func (w *WebClient) HandleDevices(wr http.ResponseWriter, r *http.Request) {
	devices := w.server.GetRegistry().List()
	w.templates.RenderPage(wr, "devices", map[string]interface{}{
		"Devices": devices,
	})
}

func (w *WebClient) HandleDeviceDetail(wr http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := w.server.GetRegistry().Get(id)
	if !ok {
		http.NotFound(wr, r)
		slog.Warn("Device id not found in registry", "id", id)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		clone, err := w.templates.ExtendedTemplates("devices")
		if err != nil {
			slog.Error("Error when extending templates", "page", "devices")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		devices := w.server.GetRegistry().List()
		clone.RenderPage(wr, "device_detail", map[string]interface{}{
			"Device":  device.Meta(),
			"Devices": devices,
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("device_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "device_detail")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(wr, "content", map[string]interface{}{
			"Device": device.Meta(),
		})
	}
}

func (w *WebClient) HandleFeatures(wr http.ResponseWriter, r *http.Request) {
	w.templates.RenderPage(wr, "features", map[string]any{
		"Features": w.server.GetTopicSources(),
	})
}

func (w *WebClient) HandleFeatureDetail(wr http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "name")
	topicSources := w.server.GetTopicSources()
	sourceId, ok := topicSources[topic]
	if !ok {
		http.NotFound(wr, r)
		slog.Warn("Feature not found in topicSources", "feature", topic)
		return
	}

	client, ok := w.server.GetRegistry().Get(sourceId)
	if !ok {
		http.NotFound(wr, r)
		slog.Error("Client not found in registry", "client", sourceId)
		return
	}

	// Get broker subscriptions
	subscribers := w.server.GetBroker().Subs(topic)
	var subs []server.DeviceMetadata
	for client := range subscribers {
		subs = append(subs, *client.Meta())
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		clone, err := w.templates.ExtendedTemplates("features")
		if err != nil {
			slog.Error("Error when extending templates", "page", "features")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		source, ok := w.server.GetRegistry().Get(topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registry from topic sources", "client id", topicSources[topic])
		}
		clone.RenderPage(wr, "feature_detail", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Features":      topicSources,
			"Subscriptions": subs,
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("feature_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "feature_detail")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		source, ok := w.server.GetRegistry().Get(topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registry from topic sources", "client id", topicSources[topic])
		}
		clone.Render(wr, "content", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Subscriptions": subs,
		})
	}
}

func (w *WebClient) HandleTransports(wr http.ResponseWriter, r *http.Request) {
	w.templates.RenderPage(wr, "transports", map[string]interface{}{
		"Transports": w.server.GetTransports(),
	})
}

func (w *WebClient) HandleTransportDetail(wr http.ResponseWriter, r *http.Request) {
	index := chi.URLParam(r, "i")
	i, err := strconv.Atoi(index)
	if err != nil {
		slog.Warn("Error converting transport index into int", "index", index)
		http.Error(wr, "Invalid transport index: "+index, http.StatusBadRequest)
		return
	}

	transports := w.server.GetTransports()
	if i >= len(transports) {
		http.NotFound(wr, r)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		clone, err := w.templates.ExtendedTemplates("transports")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transports")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		clone.RenderPage(wr, "transport_detail", map[string]interface{}{
			"Transports": transports,
			"Transport":  transports[i].Meta(),
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("transport_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transport_detail")
			http.Error(wr, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(wr, "content", map[string]interface{}{
			"Transport": transports[i].Meta(),
		})
	}
}

// HandleDeviceRename is an API endpoint for renaming devices
func (w *WebClient) HandleDeviceRename(wr http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(wr, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if err := w.RenameDevice(deviceID, req.Name); err != nil {
		http.Error(wr, err.Error(), http.StatusBadRequest)
		return
	}

	wr.WriteHeader(http.StatusOK)
	json.NewEncoder(wr).Encode(map[string]string{"status": "success"})
}

// HandleSendMessage allows the web UI to send messages like a client
func (w *WebClient) HandleSendMessage(wr http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string      `json:"type"`
		Topic   string      `json:"topic"`
		Payload interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(wr, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if err := w.SendMessage(req.Type, req.Topic, req.Payload); err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	wr.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(wr, "Message sent to %s/%s", req.Topic, req.Type)
}

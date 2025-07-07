package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

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

	w.templates.RenderPage(wr, "devices", map[string]interface{}{
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
		clone, err := w.templates.ExtendedTemplates("devices")
		if err != nil {
			w.handleTemplateError(wr, err, "devices")
			return
		}
		devices, err := w.services.Device.ListDevices()
		if err != nil {
			w.handleError(wr, err)
			return
		}
		clone.RenderPage(wr, "device_detail", map[string]interface{}{
			"Device":  device,
			"Devices": devices,
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("device_detail")
		if err != nil {
			w.handleTemplateError(wr, err, "device_detail")
			return
		}
		clone.Render(wr, "content", map[string]interface{}{
			"Device": device,
		})
	}
}

func (w *WebClient) HandleFeatures(wr http.ResponseWriter, r *http.Request) {
	features, err := w.services.Feature.ListFeatures()
	if err != nil {
		w.handleError(wr, err)
		return
	}

	w.templates.RenderPage(wr, "features", map[string]any{
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
		clone, err := w.templates.ExtendedTemplates("features")
		if err != nil {
			w.handleTemplateError(wr, err, "features")
			return
		}

		features, err := w.services.Feature.ListFeatures()
		if err != nil {
			w.handleError(wr, err)
			return
		}

		clone.RenderPage(wr, "feature_detail", map[string]interface{}{
			"Feature":       feature.Capability,
			"FeatureSource": feature,
			"Features":      features,
			"Subscriptions": feature.Subscribers,
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("feature_detail")
		if err != nil {
			w.handleTemplateError(wr, err, "feature_detail")
			return
		}

		clone.Render(wr, "content", map[string]interface{}{
			"Feature":       feature.Capability,
			"FeatureSource": feature,
			"Subscriptions": feature.Subscribers,
		})
	}
}

func (w *WebClient) HandleTransports(wr http.ResponseWriter, r *http.Request) {
	transports, err := w.services.Transport.ListTransports()
	if err != nil {
		w.handleError(wr, err)
		return
	}

	w.templates.RenderPage(wr, "transports", map[string]interface{}{
		"Transports": transports,
	})
}

func (w *WebClient) HandleTransportDetail(wr http.ResponseWriter, r *http.Request) {
	index := chi.URLParam(r, "i")
	i, err := strconv.Atoi(index)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	transport, err := w.services.Transport.GetTransport(i)
	if err != nil {
		w.handleError(wr, err)
		return
	}

	if _, ok := r.Header["Hx-Request"]; !ok {
		clone, err := w.templates.ExtendedTemplates("transports")
		if err != nil {
			w.handleTemplateError(wr, err, "transports")
			return
		}

		transports, err := w.services.Transport.ListTransports()
		if err != nil {
			w.handleError(wr, err)
			return
		}

		clone.RenderPage(wr, "transport_detail", map[string]interface{}{
			"Transports": transports,
			"Transport":  transport,
		})
	} else {
		clone, err := w.templates.ExtendedTemplates("transport_detail")
		if err != nil {
			w.handleTemplateError(wr, err, "transport_detail")
			return
		}
		clone.Render(wr, "content", map[string]interface{}{
			"Transport": transport,
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

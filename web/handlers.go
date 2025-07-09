package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

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
		w.templates.pages["devices"].Render(wr, "content", map[string]interface{}{
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
		w.templates.pages["features"].Render(wr, "content", map[string]interface{}{
			"Feature":       feature.Feature,
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
		w.templates.pages["transports"].Render(wr, "content", map[string]interface{}{
			"Transport": transport,
		})
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

	w.templates.pages["devices"].Render(wr, "rename-form", map[string]interface{}{
		"Device": device,
	})
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

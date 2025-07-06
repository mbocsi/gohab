package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/proto"
)

type Templates struct {
	templates *template.Template
}

func NewTemplates(pattern string) *Templates {
	templates := template.New("").Funcs(TemplateFuncs())
	return &Templates{
		templates: template.Must(templates.ParseGlob(filepath.Clean(pattern))),
	}
}

func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"unixTime": func(ts int64) string {
			return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
		},
		"jsonPretty": func(data []byte) string {
			var out interface{}
			_ = json.Unmarshal(data, &out)
			pretty, _ := json.MarshalIndent(out, "", "  ")
			return string(pretty)
		},
		"join": strings.Join,
	}
}

func (t *Templates) ExtendedTemplates(page string) (*Templates, error) {
	clone, err := t.templates.Clone()
	if err != nil {
		return nil, err
	}
	_, err = clone.ParseFiles(fmt.Sprintf("templates/%s.html", page))
	if err != nil {
		return nil, err
	}
	return &Templates{clone}, nil
}

func (t *Templates) Render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (t *Templates) RenderPage(w http.ResponseWriter, page string, data interface{}) {
	clone, err := t.ExtendedTemplates(page)
	if err != nil {
		http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	clone.Render(w, "layout", data)
}

func (c *Coordinator) HandleDeviceDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := c.Registery.Get(id)
	if !ok {
		http.NotFound(w, r)
		slog.Warn("Device id not found in registery", "id", id)
		return
	}
	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := c.Templates.ExtendedTemplates("devices")
		if err != nil {
			slog.Error("Error when extending templates", "page", "devices")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		devices := c.Registery.List()
		clone.RenderPage(w, "device_detail", map[string]interface{}{
			"Device":  device.Meta(),
			"Devices": devices,
		})
	} else {
		clone, err := c.Templates.ExtendedTemplates("device_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "device_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(w, "content", map[string]interface{}{
			"Device": device.Meta(),
		})
	}
}

func (c *Coordinator) HandleDevices(w http.ResponseWriter, r *http.Request) {
	devices := c.Registery.List()
	c.Templates.RenderPage(w, "devices", map[string]interface{}{
		"Devices": devices,
	})
}
func (c *Coordinator) HandleFeatures(w http.ResponseWriter, r *http.Request) {
	c.topicSourcesMu.RLock()
	c.Templates.RenderPage(w, "features", map[string]any{
		"Features": c.topicSources,
	})
	c.topicSourcesMu.RUnlock()
}

func (c *Coordinator) HandleFeatureDetail(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "name")
	c.topicSourcesMu.RLock()
	sourceId, ok := c.topicSources[topic]
	c.topicSourcesMu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		slog.Warn("Feature not found in topicSources", "feature", topic)
		return
	}

	client, ok := c.Registery.Get(sourceId)
	if !ok {
		http.NotFound(w, r)
		slog.Error("Client not found in registery", "client", sourceId)
		return
	}
	c.Broker.mu.RLock()
	subs := slices.Collect(maps.Keys(c.Broker.subs[topic]))
	c.Broker.mu.RUnlock()

	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := c.Templates.ExtendedTemplates("features")
		if err != nil {
			slog.Error("Error when extending templates", "page", "features")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		c.topicSourcesMu.RLock()
		source, ok := c.Registery.Get(c.topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registery from topic sources", "client id", c.topicSources[topic])
		}
		clone.RenderPage(w, "feature_detail", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Features":      c.topicSources,
			"Subscriptions": subs,
		})
		c.topicSourcesMu.RUnlock()
	} else {
		clone, err := c.Templates.ExtendedTemplates("feature_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "feature_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		c.topicSourcesMu.RLock()
		source, ok := c.Registery.Get(c.topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registery from topic sources", "client id", c.topicSources[topic])
		}
		clone.Render(w, "content", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Subscriptions": subs,
		})
		c.topicSourcesMu.RUnlock()
	}
}

func (c *Coordinator) HandleTransports(w http.ResponseWriter, r *http.Request) {
	c.Templates.RenderPage(w, "transports", map[string]interface{}{
		"Transports": c.Transports,
	})
}

func (c *Coordinator) HandleTransportDetail(w http.ResponseWriter, r *http.Request) {
	index := chi.URLParam(r, "i")
	i, err := strconv.Atoi(index)
	if err != nil {
		slog.Warn("Error converting transport index into int", "index", index)
		http.Error(w, "Invalid transport index: "+index, http.StatusBadRequest)
		return
	}
	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := c.Templates.ExtendedTemplates("transports")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transports")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		c.topicSourcesMu.RLock()
		clone.RenderPage(w, "transport_detail", map[string]interface{}{
			"Transports": c.Transports,
			"Transport":  c.Transports[i].Meta(),
		})
		c.topicSourcesMu.RUnlock()
	} else {
		clone, err := c.Templates.ExtendedTemplates("transport_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transport_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(w, "content", map[string]interface{}{
			"Transport": c.Transports[i].Meta(),
		})
	}
}

func (c *Coordinator) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	method := chi.URLParam(r, "method")

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	binPayload, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	msg := proto.Message{
		Type:    method,
		Topic:   topic,
		Payload: binPayload,
	}

	c.Handle(msg)

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "Message sent to %s/%s", topic, method)
}


func (c *Coordinator) HandleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/devices", http.StatusMovedPermanently)
}

package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Templates struct {
	templates *template.Template
}

func NewTemplates(pattern string) *Templates {
	funcMap := template.FuncMap{
		"join": strings.Join, // lowercase
	}
	templates := template.New("").Funcs(funcMap)
	return &Templates{
		templates: template.Must(templates.ParseGlob(filepath.Clean(pattern))),
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
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
		}
		devices := c.Registery.List()
		clone.RenderPage(w, "device_detail", map[string]interface{}{
			"Device":  device.Meta(),
			"Devices": devices,
		})
	} else {
		clone, err := c.Templates.ExtendedTemplates("device_detail")
		if err != nil {
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
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
	c.Templates.Render(w, "features", map[string]any{
		"Features": c.topicSources,
	})
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

	c.Templates.Render(w, "feature_detail", map[string]any{
		"Feature":       client.Meta().Capabilities[topic],
		"Subscriptions": subs,
	})
}

func (c *Coordinator) HandleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/devices", http.StatusMovedPermanently)
}

func (c *Coordinator) HandleTransports(w http.ResponseWriter, r *http.Request) {
	c.Templates.Render(w, "transports", map[string]interface{}{
		"Transports": c.Transports,
	})
}

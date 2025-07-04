package server

import (
	"html/template"
	"net/http"
	"path/filepath"
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

func (t *Templates) Render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (c *Coordinator) HandleDeviceDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := c.Registery.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	c.Templates.Render(w, "device_detail", map[string]interface{}{
		"Device": device.Meta(),
	})
}

func (c *Coordinator) HandleHome(w http.ResponseWriter, r *http.Request) {
	devices := c.Registery.List()
	c.Templates.Render(w, "index", map[string]interface{}{
		"Devices":      devices,
		"TopicSources": c.topicSources,
	})
}

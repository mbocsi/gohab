package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Templates struct {
	base  *template.Template
	pages map[string]*PageTemplate
}

type PageTemplate struct {
	*template.Template
}

func NewTemplates(pattern string) *Templates {
	base := template.New("").Funcs(TemplateFuncs())
	base = template.Must(base.ParseGlob(filepath.Clean(pattern)))

	// Pre-compile all page templates from the pages directory structure
	pages := make(map[string]*PageTemplate)

	// Dynamically discover page groups by reading directories
	pagesDir := "templates/pages"
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to read pages directory: %v", err))
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		group := entry.Name()
		clone, err := base.Clone()
		if err != nil {
			panic(fmt.Sprintf("Failed to clone template for %s: %v", group, err))
		}
		_, err = clone.ParseGlob(fmt.Sprintf("templates/pages/%s/*.html", group))
		if err != nil {
			panic(fmt.Sprintf("Failed to parse templates for %s: %v", group, err))
		}
		pages[group] = &PageTemplate{Template: clone}
	}

	return &Templates{
		base:  base,
		pages: pages,
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

// Renders partial html (defined template blocks)
func (pt *PageTemplate) Render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := pt.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

// Renders entire page
func (pt *PageTemplate) RenderPage(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := pt.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

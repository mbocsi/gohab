package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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
	pagesDir := filepath.Join(getProjectRoot(), "templates", "pages")
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
		_, err = clone.ParseGlob(filepath.Join(getProjectRoot(), "templates", "pages", group, "*.html"))
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

// getProjectRoot finds the project root directory by looking for go.mod
func getProjectRoot() string {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get current working directory: %v", err))
	}

	// Walk up directories looking for go.mod
	for {
		// Check if go.mod exists in current directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory without finding go.mod
			break
		}
		dir = parent
	}

	// Fallback: try to get executable path and work backwards
	executable, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(executable)
		// If executable is in bin/ directory, go up one level
		if filepath.Base(execDir) == "bin" {
			return filepath.Dir(execDir)
		}
		// Otherwise assume executable is in project root
		return execDir
	}

	// Final fallback: use current directory
	cwd, _ := os.Getwd()
	return cwd
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
func (pt *PageTemplate) Render(w io.Writer, name string, data interface{}) error {
	return pt.ExecuteTemplate(w, name, data)
}

// Renders entire page
func (pt *PageTemplate) RenderPage(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := pt.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

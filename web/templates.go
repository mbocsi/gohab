package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"
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
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"smetchic/model"
)

//go:embed templates/*.html
var templateFiles embed.FS

type Server struct {
	machine   model.Machine
	templates *template.Template
}

func NewHandler(machine model.Machine) (http.Handler, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить HTML-шаблоны: %w", err)
	}

	server := &Server{machine: machine, templates: templates}
	mux := http.NewServeMux()
	mux.Handle("/", server)
	return mux, nil
}

func (s *Server) render(w http.ResponseWriter, name string, status int, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("не удалось отобразить шаблон %q: %v", name, err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, status int, message string) {
	s.render(w, "index.html", status, indexPage{Error: message})
}

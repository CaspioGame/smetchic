package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"smetchic/equipment"
	"smetchic/model"
)

//go:embed templates/*.html
var templateFiles embed.FS

type Server struct {
	catalog         *equipment.Catalog
	defaultSettings model.EstimateSettings
	templates       *template.Template
}

func NewHandler(catalog *equipment.Catalog, defaults model.EstimateSettings) (http.Handler, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить HTML-шаблоны: %w", err)
	}
	if _, err := catalog.GetByID(defaults.EquipmentID); err != nil {
		return nil, fmt.Errorf("оборудование по умолчанию не найдено: %w", err)
	}

	server := &Server{catalog: catalog, defaultSettings: defaults, templates: templates}
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

package web

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"smetchic/application"
	"smetchic/equipment"
	"smetchic/model"
	"smetchic/storage"
)

//go:embed templates/*.html
var templateFiles embed.FS

type Server struct {
	catalog         *equipment.Catalog
	store           *storage.Store
	companyID       string
	company         model.Company
	lifecycle       *application.Service
	defaultSettings model.EstimateSettings
	templates       *template.Template
	csrfToken       string
}

func NewLifecycleCabinetHandler(store *storage.Store, company model.Company, lifecycle *application.Service) (http.Handler, error) {
	legacyCompanyID := company.ID
	defaults := model.EstimateSettings{}
	if store != nil {
		var err error
		legacyCompanyID, err = store.FirstCompanyID()
		if err != nil {
			return nil, fmt.Errorf("не удалось определить компанию JSON-кабинета: %w", err)
		}
		snapshot, err := store.Snapshot(legacyCompanyID)
		if err != nil {
			return nil, fmt.Errorf("компания JSON-кабинета не найдена: %w", err)
		}
		defaults.MarkupPercent = snapshot.Settings.DefaultMarkupPercent
		for _, item := range snapshot.Equipment {
			if item.Status == model.EquipmentStatusActive {
				defaults.EquipmentID = item.ID
				break
			}
		}
	}
	server, err := newServer(nil, store, legacyCompanyID, defaults)
	if err != nil {
		return nil, err
	}
	server.company = company
	server.lifecycle = lifecycle
	return server, nil
}

func NewHandler(catalog *equipment.Catalog, defaults model.EstimateSettings) (http.Handler, error) {
	server, err := newServer(catalog, nil, "", defaults)
	if err != nil {
		return nil, err
	}
	if _, err := catalog.GetByID(defaults.EquipmentID); err != nil {
		return nil, fmt.Errorf("оборудование по умолчанию не найдено: %w", err)
	}
	return server, nil
}

func NewCabinetHandler(store *storage.Store, companyID string) (http.Handler, error) {
	snapshot, err := store.Snapshot(companyID)
	if err != nil {
		return nil, fmt.Errorf("компания кабинета не найдена: %w", err)
	}
	defaults := model.EstimateSettings{MarkupPercent: snapshot.Settings.DefaultMarkupPercent}
	for _, item := range snapshot.Equipment {
		if item.Status == model.EquipmentStatusActive {
			defaults.EquipmentID = item.ID
			break
		}
	}
	server, err := newServer(nil, store, companyID, defaults)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func newServer(catalog *equipment.Catalog, store *storage.Store, companyID string, defaults model.EstimateSettings) (*Server, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить HTML-шаблоны: %w", err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("не удалось подготовить защиту форм: %w", err)
	}
	return &Server{
		catalog: catalog, store: store, companyID: companyID,
		defaultSettings: defaults, templates: templates,
		csrfToken: base64.RawURLEncoding.EncodeToString(token),
	}, nil
}

func (s *Server) render(w http.ResponseWriter, name string, status int, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("не удалось отобразить шаблон %q: %v", name, err)
	}
}

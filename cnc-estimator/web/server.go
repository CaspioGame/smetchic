package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"smetchic/application"
	"smetchic/equipment"
	"smetchic/model"
	"smetchic/storage"
)

//go:embed templates/*.html
var templateFiles embed.FS

type Server struct {
	chatAssistant   func(context.Context, string, map[string]string, chatCatalog) (estimateAssistantReply, error)
	catalog         *equipment.Catalog
	store           *storage.Store
	companyID       string
	company         model.Company
	lifecycle       *application.Service
	authentication  *application.AuthenticationService
	sessions        *sessionStore
	defaultSettings model.EstimateSettings
	templates       *template.Template
	csrfToken       string
}

func NewLifecycleCabinetHandler(store *storage.Store, company model.Company, lifecycle *application.Service) (http.Handler, error) {
	server, err := newLifecycleServer(store, company, lifecycle)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func NewAuthenticatedLifecycleCabinetHandler(
	_ *storage.Store,
	lifecycle *application.Service,
	authentication *application.AuthenticationService,
) (http.Handler, error) {
	if lifecycle == nil {
		return nil, fmt.Errorf("PostgreSQL lifecycle service is required")
	}
	if authentication == nil {
		return nil, fmt.Errorf("authentication service is required")
	}
	server, err := newLifecycleServer(nil, model.Company{}, lifecycle)
	if err != nil {
		return nil, err
	}
	server.authentication = authentication
	server.sessions = newSessionStore()
	return server, nil
}

func newLifecycleServer(store *storage.Store, company model.Company, lifecycle *application.Service) (*Server, error) {
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
	templates, err := template.New("root").Funcs(template.FuncMap{
		"money":           formatMoney,
		"equipmentType":   equipmentTypeLabel,
		"estimateModules": application.EstimateModuleCatalog,
	}).ParseFS(templateFiles, "templates/*.html")
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

func formatMoney(amount float64, currency string) string {
	formatted := strconv.FormatFloat(amount, 'f', 2, 64)
	parts := strings.SplitN(formatted, ".", 2)
	integer := parts[0]
	start := 0
	if strings.HasPrefix(integer, "-") {
		start = 1
	}
	for index := len(integer) - 3; index > start; index -= 3 {
		integer = integer[:index] + " " + integer[index:]
	}
	symbol := map[string]string{
		"RUB": "₽", "USD": "$", "EUR": "€", "CNY": "¥",
	}[currency]
	if symbol == "" {
		symbol = currency
	}
	return integer + "." + parts[1] + " " + symbol
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, status int, data any) {
	data = s.applyPageSecurity(r, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("не удалось отобразить шаблон %q: %v", name, err)
	}
}

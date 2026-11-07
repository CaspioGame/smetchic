package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"smetchic/application"
	"smetchic/model"
)

type consumableForm struct {
	ID            string
	Name          string
	Manufacturer  string
	ToolType      string
	DiameterMm    string
	FluteCount    string
	ToolMaterial  string
	Coating       string
	LengthMm      string
	StockQuantity string
	UnitPrice     string
	Currency      string
}

type orderForm struct {
	ID          string
	ProjectID   string
	Name        string
	Description string
	Status      string
}

type orderListRow struct {
	Order      application.OrderRecord
	StatusName string
}

type workspacePage struct {
	pageBase
	View                string
	Company             model.Company
	Settings            model.CompanySettings
	Users               []application.CompanyUserRecord
	EquipmentRows       []equipmentListRow
	Equipment           []model.Equipment
	EquipmentModel      []model.EquipmentModel
	Materials           []model.Material
	GlobalMaterials     []model.GlobalMaterial
	GlobalMaterial      model.GlobalMaterial
	GlobalMaterialQuery string
	Consumables         []model.Consumable
	Orders              []application.OrderRecord
	OrderRows           []orderListRow
	Projects            []application.ProjectRecord
	EquipmentForm       equipmentForm
	MaterialForm        materialForm
	ConsumableForm      consumableForm
	OrderForm           orderForm
}

func (s *Server) serveWorkspace(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.URL.Path == "/company":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderCompanyWorkspace(w, r)
	case r.URL.Path == "/company/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveCompanyWorkspace(w, r)
	case r.URL.Path == "/equipment":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderEquipmentWorkspace(w, r)
	case r.URL.Path == "/equipment/new":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderEquipmentWorkspaceForm(w, r, true)
	case r.URL.Path == "/equipment/edit":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderEquipmentWorkspaceForm(w, r, false)
	case r.URL.Path == "/equipment/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveEquipmentWorkspace(w, r)
	case r.URL.Path == "/equipment/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deleteEquipmentWorkspace(w, r)
	case r.URL.Path == "/materials":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderMaterialsWorkspace(w, r)
	case r.URL.Path == "/materials/new":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderMaterialWorkspaceForm(w, r, true)
	case r.URL.Path == "/materials/edit":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderMaterialWorkspaceForm(w, r, false)
	case r.URL.Path == "/materials/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveMaterialWorkspace(w, r)
	case r.URL.Path == "/materials/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deleteMaterialWorkspace(w, r)
	case r.URL.Path == "/consumables":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderConsumablesWorkspace(w, r)
	case r.URL.Path == "/consumables/new":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderConsumableWorkspaceForm(w, r, true)
	case r.URL.Path == "/consumables/edit":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderConsumableWorkspaceForm(w, r, false)
	case r.URL.Path == "/consumables/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveConsumableWorkspace(w, r)
	case r.URL.Path == "/consumables/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deleteConsumableWorkspace(w, r)
	case r.URL.Path == "/catalog/equipment":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderGlobalEquipment(w, r)
	case r.URL.Path == "/catalog/materials":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderGlobalMaterials(w, r)
	case strings.HasPrefix(r.URL.Path, "/catalog/materials/"):
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderGlobalMaterial(w, r, strings.TrimPrefix(r.URL.Path, "/catalog/materials/"))
	case r.URL.Path == "/orders":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderOrders(w, r)
	case r.URL.Path == "/orders/new":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderOrderForm(w, r, application.OrderRecord{}, http.StatusOK)
	case r.URL.Path == "/orders/create":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.createOrder(w, r)
	case r.URL.Path == "/orders/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.updateOrder(w, r)
	case strings.HasPrefix(r.URL.Path, "/orders/"):
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderOrder(w, r, strings.TrimPrefix(r.URL.Path, "/orders/"))
	default:
		return false
	}
	return true
}

func (s *Server) workspaceMethodNotAllowed(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Allow", "GET, POST")
	http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
	return true
}

func (s *Server) renderCompanyWorkspace(w http.ResponseWriter, r *http.Request) {
	companyID := companyIDForRequest(s, r)
	company, err := s.lifecycle.GetCompany(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "load company profile", err)
		return
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "load company settings", err)
		return
	}
	users, err := s.lifecycle.ListCompanyUsers(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "list company users", err)
		return
	}
	identity, _ := identityFromRequest(r)
	page := s.workspaceBase(r, "Профиль компании", "company", "company")
	page.Company, page.Settings, page.Users = company, settings, users
	page.UserName = identity.User.Name
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) saveCompanyWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	companyID := companyIDForRequest(s, r)
	company, err := s.lifecycle.GetCompany(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "load company before save", err)
		return
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "load settings before save", err)
		return
	}
	company.Name = strings.TrimSpace(r.FormValue("name"))
	settings.Currency = strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	settings.DefaultMarkupPercent, err = parseRequiredNumber("Наценка по умолчанию", r.FormValue("default_markup_percent"))
	if err != nil {
		s.renderCompanyError(w, r, company, settings, "Проверьте значения полей компании.")
		return
	}
	if err := s.lifecycle.UpdateCompany(r.Context(), company, settings); err != nil {
		s.renderCompanyError(w, r, company, settings, applicationPublicError(err))
		return
	}
	http.Redirect(w, r, "/company?saved=1", http.StatusSeeOther)
}

func (s *Server) renderCompanyError(w http.ResponseWriter, r *http.Request, company model.Company, settings model.CompanySettings, message string) {
	users, err := s.lifecycle.ListCompanyUsers(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "reload company users", err)
		return
	}
	page := s.workspaceBase(r, "Профиль компании", "company", "company")
	page.Company, page.Settings, page.Users, page.Error = company, settings, users, message
	s.render(w, r, "workspace.html", http.StatusBadRequest, page)
}

func (s *Server) renderEquipmentWorkspace(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "list company equipment", err)
		return
	}
	rows := make([]equipmentListRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, equipmentListRow{
			Equipment: item, TypeName: equipmentTypeLabel(item.EquipmentModel.Type),
			StatusName: equipmentStatusLabel(item.Status),
		})
	}
	page := s.workspaceBase(r, "Моё оборудование", "equipment", "equipment")
	page.EquipmentRows = rows
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderEquipmentWorkspaceForm(w http.ResponseWriter, r *http.Request, create bool) {
	models, err := s.lifecycle.ListEquipmentModels(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "load global equipment models", err)
		return
	}
	page := s.workspaceBase(r, "Добавить оборудование", "equipment", "equipment-form")
	page.EquipmentModel = models
	page.EquipmentForm = equipmentForm{
		Status: string(model.EquipmentStatusActive), MachineRatePerHour: "1500",
	}
	if !create {
		page.Title = "Редактировать оборудование"
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		item, err := s.lifecycle.GetEquipment(r.Context(), companyIDForRequest(s, r), id)
		if err != nil {
			s.lifecycleError(w, r, "load company equipment for edit", err)
			return
		}
		page.EquipmentForm = equipmentForm{
			ID: item.ID, Name: item.Name, EquipmentModelID: item.EquipmentModelID,
			InventoryNumber: item.InventoryNumber, SerialNumber: item.SerialNumber,
			Status: string(item.Status), MachineRatePerHour: formatNumber(item.MachineRatePerHour),
		}
	}
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) saveEquipmentWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := equipmentForm{
		ID: strings.TrimSpace(r.FormValue("id")), Name: strings.TrimSpace(r.FormValue("name")),
		EquipmentModelID:   strings.TrimSpace(r.FormValue("equipment_model_id")),
		InventoryNumber:    strings.TrimSpace(r.FormValue("inventory_number")),
		SerialNumber:       strings.TrimSpace(r.FormValue("serial_number")),
		Status:             strings.TrimSpace(r.FormValue("status")),
		MachineRatePerHour: strings.TrimSpace(r.FormValue("machine_rate_per_hour")),
	}
	rate, err := parseRequiredNumber("Ставка за час", form.MachineRatePerHour)
	if err != nil {
		s.renderEquipmentError(w, r, form, err.Error())
		return
	}
	item, err := s.lifecycle.SaveEquipment(r.Context(), companyIDForRequest(s, r), model.Equipment{
		ID: form.ID, Name: form.Name, EquipmentModelID: form.EquipmentModelID,
		InventoryNumber: form.InventoryNumber, SerialNumber: form.SerialNumber,
		Status: model.EquipmentStatus(form.Status), MachineRatePerHour: rate,
	})
	if err != nil {
		s.renderEquipmentError(w, r, form, applicationPublicError(err))
		return
	}
	http.Redirect(w, r, "/equipment?saved=1&id="+item.ID, http.StatusSeeOther)
}

func (s *Server) renderEquipmentError(w http.ResponseWriter, r *http.Request, form equipmentForm, message string) {
	models, err := s.lifecycle.ListEquipmentModels(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "reload equipment models", err)
		return
	}
	page := s.workspaceBase(r, "Оборудование", "equipment", "equipment-form")
	page.Error, page.EquipmentForm, page.EquipmentModel = message, form, models
	s.render(w, r, "workspace.html", http.StatusBadRequest, page)
}

func (s *Server) deleteEquipmentWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteEquipment(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		if errors.Is(err, application.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("delete company equipment: %v", err)
		s.renderEquipmentDeleteError(w, r, "Оборудование не удалось удалить: возможно, оно используется в сохранённой смете.")
		return
	}
	http.Redirect(w, r, "/equipment?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderEquipmentDeleteError(w http.ResponseWriter, r *http.Request, message string) {
	items, err := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "reload equipment after delete error", err)
		return
	}
	rows := make([]equipmentListRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, equipmentListRow{Equipment: item, TypeName: equipmentTypeLabel(item.EquipmentModel.Type), StatusName: equipmentStatusLabel(item.Status)})
	}
	page := s.workspaceBase(r, "Моё оборудование", "equipment", "equipment")
	page.Error, page.EquipmentRows = message, rows
	s.render(w, r, "workspace.html", http.StatusConflict, page)
}

func (s *Server) renderMaterialsWorkspace(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "list company materials", err)
		return
	}
	page := s.workspaceBase(r, "Мои материалы", "materials", "materials")
	page.Materials = items
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderMaterialWorkspaceForm(w http.ResponseWriter, r *http.Request, create bool) {
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "load settings for material form", err)
		return
	}
	page := s.workspaceBase(r, "Добавить материал", "materials", "material-form")
	page.MaterialForm = materialForm{Category: string(model.MaterialCategoryAluminum), Currency: settings.Currency}
	if !create {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		items, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
		if err != nil {
			s.workspaceInternalError(w, "load material for edit", err)
			return
		}
		found := false
		for _, item := range items {
			if item.ID == id {
				page.Title = "Редактировать материал"
				page.MaterialForm = materialForm{
					ID: item.ID, Name: item.Name, Grade: item.Grade, Category: string(item.Category),
					Density: formatNumber(item.Density), PricePerKg: formatNumber(item.PricePerKg), Currency: item.Currency,
				}
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
	}
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) saveMaterialWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := materialForm{
		ID: strings.TrimSpace(r.FormValue("id")), Name: strings.TrimSpace(r.FormValue("name")),
		Grade: strings.TrimSpace(r.FormValue("grade")), Category: strings.TrimSpace(r.FormValue("category")),
		Density: strings.TrimSpace(r.FormValue("density")), PricePerKg: strings.TrimSpace(r.FormValue("price_per_kg")),
		Currency: strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	}
	density, err := parseRequiredNumber("Плотность", form.Density)
	if err != nil {
		s.renderMaterialError(w, r, form, err.Error())
		return
	}
	price, err := parseRequiredNumber("Цена за кг", form.PricePerKg)
	if err != nil {
		s.renderMaterialError(w, r, form, err.Error())
		return
	}
	_, err = s.lifecycle.SaveMaterial(r.Context(), companyIDForRequest(s, r), model.Material{
		ID: form.ID, Name: form.Name, Grade: form.Grade,
		Category: model.MaterialCategory(form.Category), Density: density,
		PricePerKg: price, Currency: form.Currency,
	})
	if err != nil {
		s.renderMaterialError(w, r, form, applicationPublicError(err))
		return
	}
	http.Redirect(w, r, "/materials?saved=1", http.StatusSeeOther)
}

func (s *Server) renderMaterialError(w http.ResponseWriter, r *http.Request, form materialForm, message string) {
	page := s.workspaceBase(r, "Материал", "materials", "material-form")
	page.Error, page.MaterialForm = message, form
	s.render(w, r, "workspace.html", http.StatusBadRequest, page)
}

func (s *Server) deleteMaterialWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteMaterial(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		log.Printf("delete company material: %v", err)
		s.renderMaterialDeleteError(w, r, "Материал не удалось удалить. Возможно, на него ссылаются заготовки.")
		return
	}
	http.Redirect(w, r, "/materials?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderMaterialDeleteError(w http.ResponseWriter, r *http.Request, message string) {
	items, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "reload materials after delete error", err)
		return
	}
	page := s.workspaceBase(r, "Мои материалы", "materials", "materials")
	page.Error, page.Materials = message, items
	s.render(w, r, "workspace.html", http.StatusConflict, page)
}

func (s *Server) renderConsumablesWorkspace(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "list company consumables", err)
		return
	}
	page := s.workspaceBase(r, "Мои расходники", "consumables", "consumables")
	page.Consumables = items
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderConsumableWorkspaceForm(w http.ResponseWriter, r *http.Request, create bool) {
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "load settings for consumable form", err)
		return
	}
	page := s.workspaceBase(r, "Добавить фрезу", "consumables", "consumable-form")
	page.ConsumableForm = consumableForm{Currency: settings.Currency}
	if create {
		page.ConsumableForm.StockQuantity = "0"
		page.ConsumableForm.UnitPrice = "0"
	} else {
		items, err := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
		if err != nil {
			s.workspaceInternalError(w, "load consumable for edit", err)
			return
		}
		id, found := strings.TrimSpace(r.URL.Query().Get("id")), false
		for _, item := range items {
			if item.ID == id {
				page.Title = "Редактировать фрезу"
				page.ConsumableForm = consumableForm{
					ID: item.ID, Name: item.Name, Manufacturer: item.Manufacturer, ToolType: item.ToolType,
					DiameterMm: formatNumber(item.DiameterMm), FluteCount: strconv.Itoa(item.FluteCount),
					ToolMaterial: item.ToolMaterial, Coating: item.Coating, LengthMm: formatNumber(item.LengthMm),
					StockQuantity: strconv.Itoa(item.StockQuantity), UnitPrice: formatNumber(item.UnitPrice), Currency: item.Currency,
				}
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
	}
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) saveConsumableWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := consumableForm{
		ID: strings.TrimSpace(r.FormValue("id")), Name: strings.TrimSpace(r.FormValue("name")),
		Manufacturer: strings.TrimSpace(r.FormValue("manufacturer")), ToolType: strings.TrimSpace(r.FormValue("tool_type")),
		DiameterMm: strings.TrimSpace(r.FormValue("diameter_mm")), FluteCount: strings.TrimSpace(r.FormValue("flute_count")),
		ToolMaterial: strings.TrimSpace(r.FormValue("tool_material")), Coating: strings.TrimSpace(r.FormValue("coating")),
		LengthMm: strings.TrimSpace(r.FormValue("length_mm")), StockQuantity: strings.TrimSpace(r.FormValue("stock_quantity")),
		UnitPrice: strings.TrimSpace(r.FormValue("unit_price")), Currency: strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	}
	diameter, err := parseRequiredNumber("Диаметр", form.DiameterMm)
	if err != nil {
		s.renderConsumableError(w, r, form, err.Error())
		return
	}
	length, err := parseRequiredNumber("Длина", form.LengthMm)
	if err != nil {
		s.renderConsumableError(w, r, form, err.Error())
		return
	}
	flutes, err := strconv.Atoi(form.FluteCount)
	if err != nil {
		s.renderConsumableError(w, r, form, "Укажите корректное количество зубьев.")
		return
	}
	quantity, err := strconv.Atoi(form.StockQuantity)
	if err != nil {
		s.renderConsumableError(w, r, form, "Укажите корректное количество на складе.")
		return
	}
	price, err := parseRequiredNumber("Цена за штуку", form.UnitPrice)
	if err != nil {
		s.renderConsumableError(w, r, form, err.Error())
		return
	}
	_, err = s.lifecycle.SaveConsumable(r.Context(), companyIDForRequest(s, r), model.Consumable{
		ID: form.ID, CompanyID: companyIDForRequest(s, r), Category: model.ConsumableCutter,
		Name: form.Name, Manufacturer: form.Manufacturer, ToolType: form.ToolType,
		DiameterMm: diameter, FluteCount: flutes, ToolMaterial: form.ToolMaterial,
		Coating: form.Coating, LengthMm: length, StockQuantity: quantity,
		UnitPrice: price, Currency: form.Currency, Attributes: json.RawMessage(`{}`),
	})
	if err != nil {
		s.renderConsumableError(w, r, form, applicationPublicError(err))
		return
	}
	http.Redirect(w, r, "/consumables?saved=1", http.StatusSeeOther)
}

func (s *Server) renderConsumableError(w http.ResponseWriter, r *http.Request, form consumableForm, message string) {
	page := s.workspaceBase(r, "Фреза", "consumables", "consumable-form")
	page.Error, page.ConsumableForm = message, form
	s.render(w, r, "workspace.html", http.StatusBadRequest, page)
}

func (s *Server) deleteConsumableWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteConsumable(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		if errors.Is(err, application.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.workspaceInternalError(w, "delete company consumable", err)
		return
	}
	http.Redirect(w, r, "/consumables?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderGlobalEquipment(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListEquipmentModels(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "list global equipment models", err)
		return
	}
	page := s.workspaceBase(r, "Общая база оборудования", "catalog-equipment", "catalog-equipment")
	page.EquipmentModel = items
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderGlobalMaterials(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListGlobalMaterials(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "list global reference materials", err)
		return
	}
	page := s.workspaceBase(r, "Общая база материалов", "catalog-materials", "catalog-materials")
	page.GlobalMaterialQuery = strings.TrimSpace(r.URL.Query().Get("q"))
	if page.GlobalMaterialQuery != "" {
		query := strings.ToLower(page.GlobalMaterialQuery)
		filtered := make([]model.GlobalMaterial, 0, len(items))
		for _, item := range items {
			haystack := strings.ToLower(strings.Join([]string{
				item.Name, item.Grade, string(item.Category), item.Description,
			}, " "))
			if strings.Contains(haystack, query) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	page.GlobalMaterials = items
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderGlobalMaterial(w http.ResponseWriter, r *http.Request, materialID string) {
	if materialID == "" || strings.Contains(materialID, "/") {
		http.NotFound(w, r)
		return
	}
	items, err := s.lifecycle.ListGlobalMaterials(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "load global material details", err)
		return
	}
	for _, item := range items {
		if item.ID == materialID {
			page := s.workspaceBase(r, item.Name+" "+item.Grade, "catalog-materials", "catalog-material-detail")
			page.GlobalMaterial = item
			s.render(w, r, "workspace.html", http.StatusOK, page)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) renderOrders(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListOrders(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "list company orders", err)
		return
	}
	page := s.workspaceBase(r, "Заказы", "orders", "orders")
	page.Orders = items
	page.OrderRows = make([]orderListRow, 0, len(items))
	for _, item := range items {
		page.OrderRows = append(page.OrderRows, orderListRow{
			Order: item, StatusName: formatOrderStatus(item.Status),
		})
	}
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderOrderForm(w http.ResponseWriter, r *http.Request, order application.OrderRecord, status int) {
	projects, err := s.lifecycle.ListProjects(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "load projects for order", err)
		return
	}
	page := s.workspaceBase(r, "Новый заказ", "orders", "order-form")
	page.Projects, page.OrderForm = projects, orderForm{
		ID: order.ID, ProjectID: order.ProjectID, Name: order.Name,
		Description: order.Description, Status: order.Status,
	}
	if order.ID != "" {
		page.Title = "Редактировать заказ"
	}
	page.Error = r.URL.Query().Get("error")
	s.render(w, r, "workspace.html", status, page)
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	identity, _ := identityFromRequest(r)
	order, err := s.lifecycle.CreateOrder(
		r.Context(), identity.Company.ID, identity.User.ID,
		r.FormValue("project_id"), r.FormValue("name"), r.FormValue("description"),
	)
	if err != nil {
		s.renderOrderFormError(w, r, application.OrderRecord{
			ProjectID: r.FormValue("project_id"), Name: r.FormValue("name"),
			Description: r.FormValue("description"), Status: application.OrderStatusDraft,
		}, err)
		return
	}
	http.Redirect(w, r, "/orders/"+order.ID, http.StatusSeeOther)
}

func (s *Server) renderOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	if orderID == "" || strings.Contains(orderID, "/") {
		http.NotFound(w, r)
		return
	}
	order, err := s.lifecycle.GetOrder(r.Context(), companyIDForRequest(s, r), orderID)
	if err != nil {
		s.lifecycleError(w, r, "get company order", err)
		return
	}
	s.renderOrderForm(w, r, order, http.StatusOK)
}

func (s *Server) updateOrder(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	identity, _ := identityFromRequest(r)
	_, err := s.lifecycle.UpdateOrder(r.Context(), application.OrderRecord{
		ID: strings.TrimSpace(r.FormValue("id")), CompanyID: identity.Company.ID,
		ProjectID: r.FormValue("project_id"), Name: r.FormValue("name"),
		Description: r.FormValue("description"), Status: r.FormValue("status"),
	})
	if err != nil {
		order := application.OrderRecord{
			ID: r.FormValue("id"), ProjectID: r.FormValue("project_id"),
			Name: r.FormValue("name"), Description: r.FormValue("description"),
			Status: r.FormValue("status"),
		}
		s.renderOrderFormError(w, r, order, err)
		return
	}
	http.Redirect(w, r, "/orders/"+r.FormValue("id")+"?saved=1", http.StatusSeeOther)
}

func (s *Server) renderOrderFormError(w http.ResponseWriter, r *http.Request, order application.OrderRecord, err error) {
	pageStatus := http.StatusBadRequest
	message := applicationPublicError(err)
	if !isLifecycleValidationError(err) && !errors.Is(err, application.ErrNotFound) {
		pageStatus, message = http.StatusInternalServerError, "Не удалось сохранить заказ. Проверьте проект и повторите попытку."
	}
	page := s.workspaceBase(r, "Заказ", "orders", "order-form")
	page.Error = message
	projects, projectsErr := s.lifecycle.ListProjects(r.Context(), companyIDForRequest(s, r))
	if projectsErr != nil {
		s.workspaceInternalError(w, "reload projects after order error", projectsErr)
		return
	}
	page.Projects = projects
	page.OrderForm = orderForm{
		ID: order.ID, ProjectID: order.ProjectID, Name: order.Name,
		Description: order.Description, Status: order.Status,
	}
	s.render(w, r, "workspace.html", pageStatus, page)
}

func (s *Server) workspaceBase(r *http.Request, title, active, view string) workspacePage {
	identity, _ := identityFromRequest(r)
	page := workspacePage{
		pageBase: pageBase{
			Title: title, Active: active, LifecycleEnabled: true,
			Authenticated: true, UserName: identity.User.Name,
			Flash: flashMessage(r),
		},
		View:    view,
		Company: identity.Company,
	}
	return page
}

func (s *Server) workspaceInternalError(w http.ResponseWriter, action string, err error) {
	log.Printf("%s: %v", action, err)
	http.Error(w, "Не удалось загрузить данные. Попробуйте позже.", http.StatusInternalServerError)
}

func applicationPublicError(err error) string {
	var validationError *application.ValidationError
	if errors.As(err, &validationError) {
		return validationError.Message
	}
	if errors.Is(err, application.ErrNotFound) {
		return "Запись не найдена или недоступна в вашей компании."
	}
	log.Printf("workspace operation failed: %v", err)
	return "Не удалось сохранить данные. Проверьте заполненные поля."
}

func formatOrderStatus(status string) string {
	switch status {
	case application.OrderStatusDraft:
		return "Черновик"
	case application.OrderStatusInProgress:
		return "В работе"
	case application.OrderStatusCompleted:
		return "Завершён"
	case application.OrderStatusCancelled:
		return "Отменён"
	default:
		return fmt.Sprintf("Неизвестный статус (%s)", status)
	}
}

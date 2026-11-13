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
	ID               string
	Name             string
	Manufacturer     string
	ToolType         string
	DiameterMm       string
	FluteCount       string
	ToolMaterial     string
	Coating          string
	LengthMm         string
	StockQuantity    string
	UnitPrice        string
	ResourceMinutes  string
	UsedMinutes      string
	ResourceUnits    string
	ResourceUnitName string
	Currency         string
}

type orderForm struct {
	ID          string
	ProjectID   string
	Name        string
	Description string
	Status      string
}

type postgresStockBlankForm struct {
	ID            string
	Name          string
	MaterialID    string
	Shape         string
	Length        string
	Width         string
	Height        string
	Diameter      string
	PricePerPiece string
	Currency      string
}

type orderListRow struct {
	Order      application.OrderRecord
	StatusName string
}

type workspacePage struct {
	pageBase
	View                   string
	Company                model.Company
	Settings               model.CompanySettings
	Users                  []application.CompanyUserRecord
	EquipmentRows          []equipmentListRow
	Equipment              []model.Equipment
	EquipmentNames         map[string]string
	EquipmentModel         []model.EquipmentModel
	Materials              []model.Material
	StockBlanks            []model.StockBlank
	GlobalMaterials        []model.GlobalMaterial
	GlobalMaterial         model.GlobalMaterial
	GlobalMaterialQuery    string
	Consumables            []model.Consumable
	Orders                 []application.OrderRecord
	OrderRows              []orderListRow
	Projects               []application.ProjectRecord
	EquipmentForm          equipmentForm
	MaterialForm           materialForm
	ConsumableForm         consumableForm
	OrderForm              orderForm
	StockBlankForm         postgresStockBlankForm
	ProductionEconomics    application.CompanyProductionEconomics
	EquipmentEconomicsForm model.EquipmentEconomics
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
	case r.URL.Path == "/economics":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderEconomicsWorkspace(w, r)
	case r.URL.Path == "/economics/company":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveCompanyEconomics(w, r)
	case r.URL.Path == "/economics/equipment":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveEquipmentEconomics(w, r)
	case r.URL.Path == "/economics/equipment/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deleteEquipmentEconomics(w, r)
	case r.URL.Path == "/economics/personnel":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.savePersonnelRate(w, r)
	case r.URL.Path == "/economics/personnel/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deletePersonnelRate(w, r)
	case r.URL.Path == "/economics/overheads":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.saveCompanyOverhead(w, r)
	case r.URL.Path == "/economics/overheads/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deleteCompanyOverhead(w, r)
	case r.URL.Path == "/stock-blanks":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderPostgresStockBlanks(w, r)
	case r.URL.Path == "/stock-blanks/new":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderPostgresStockBlankForm(w, r, true)
	case r.URL.Path == "/stock-blanks/edit":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderPostgresStockBlankForm(w, r, false)
	case r.URL.Path == "/stock-blanks/save":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.savePostgresStockBlank(w, r)
	case r.URL.Path == "/stock-blanks/delete":
		if r.Method != http.MethodPost {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.deletePostgresStockBlank(w, r)
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
	case r.URL.Path == "/catalog/consumables":
		if r.Method != http.MethodGet {
			return s.workspaceMethodNotAllowed(w, r)
		}
		s.renderGlobalConsumables(w, r)
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

func (s *Server) renderEconomicsWorkspace(w http.ResponseWriter, r *http.Request) {
	page, err := s.loadEconomicsWorkspace(r)
	if err != nil {
		s.workspaceInternalError(w, "load production economics", err)
		return
	}
	if equipmentID := strings.TrimSpace(r.URL.Query().Get("equipment_id")); equipmentID != "" {
		item, getErr := s.lifecycle.GetEquipmentEconomics(r.Context(), companyIDForRequest(s, r), equipmentID)
		if getErr != nil {
			s.renderEconomicsError(w, r, "load equipment economics for edit", getErr)
			return
		}
		if item != nil {
			page.EquipmentEconomicsForm = *item
		} else {
			page.EquipmentEconomicsForm.CompanyID = companyIDForRequest(s, r)
			page.EquipmentEconomicsForm.EquipmentID = equipmentID
		}
	}
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) loadEconomicsWorkspace(r *http.Request) (workspacePage, error) {
	companyID := companyIDForRequest(s, r)
	economics, err := s.lifecycle.ListProductionEconomics(r.Context(), companyID)
	if err != nil {
		return workspacePage{}, err
	}
	equipment, err := s.lifecycle.ListEquipment(r.Context(), companyID)
	if err != nil {
		return workspacePage{}, err
	}
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyID)
	if err != nil {
		return workspacePage{}, err
	}
	stockBlanks, err := s.lifecycle.ListStockBlanks(r.Context(), companyID)
	if err != nil {
		return workspacePage{}, err
	}
	page := s.workspaceBase(r, "Производственная экономика", "economics", "economics")
	page.ProductionEconomics, page.Equipment = economics, equipment
	page.EquipmentNames = make(map[string]string, len(equipment))
	for _, item := range equipment {
		page.EquipmentNames[item.ID] = item.Name
	}
	page.Materials, page.StockBlanks = materials, stockBlanks
	return page, nil
}

func (s *Server) saveCompanyEconomics(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	rent, err := parseRequiredNumber("аренда в месяц", r.FormValue("monthly_rent"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	hours, err := parseRequiredNumber("производственные часы в месяц", r.FormValue("monthly_production_hours"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	markup, err := parseRequiredNumber("наценка", r.FormValue("default_markup_percent"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	err = s.lifecycle.SaveCompanyEconomics(r.Context(), companyIDForRequest(s, r), model.CompanyEconomics{
		DefaultMarkupPercent: markup, MonthlyRent: rent, MonthlyProductionHours: hours,
		RentAllocationMethod: model.AllocationByProductionHours,
		Currency:             strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	})
	if err != nil {
		s.renderEconomicsError(w, r, "save company production economics", err)
		return
	}
	http.Redirect(w, r, "/economics?saved=1", http.StatusSeeOther)
}

func (s *Server) saveEquipmentEconomics(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	companyID := companyIDForRequest(s, r)
	values := make([]float64, 9)
	var err error
	fields := []struct {
		name string
		key  string
	}{
		{"цена приобретения", "purchase_price"},
		{"остаточная стоимость", "residual_value"},
		{"срок службы", "useful_life_years"},
		{"доступные часы в год", "annual_available_hours"},
		{"мощность", "consumption_kw"},
		{"цена электроэнергии", "electricity_price_per_kwh"},
		{"обслуживание в год", "annual_maintenance_cost"},
		{"ремонт в год", "annual_repair_cost"},
		{"прочие расходы станка в год", "annual_other_cost"},
	}
	for index, field := range fields {
		values[index], err = parseRequiredNumber(field.name, r.FormValue(field.key))
		if err != nil {
			http.Error(w, applicationPublicError(err), http.StatusBadRequest)
			return
		}
	}
	item := model.EquipmentEconomics{
		EquipmentID: r.FormValue("equipment_id"), PurchasePrice: values[0],
		ResidualValue: values[1], UsefulLifeYears: values[2],
		AnnualAvailableHours: values[3], ConsumptionKW: values[4],
		ElectricityPricePerKWh: values[5], AnnualMaintenanceCost: values[6],
		AnnualRepairCost: values[7], AnnualOtherCost: values[8],
	}
	if err := s.lifecycle.SaveEquipmentEconomics(r.Context(), companyID, item); err != nil {
		s.renderEconomicsError(w, r, "save equipment economics", err)
		return
	}
	http.Redirect(w, r, "/economics?saved=1", http.StatusSeeOther)
}

func (s *Server) deleteEquipmentEconomics(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteEquipmentEconomics(r.Context(), companyIDForRequest(s, r), r.FormValue("equipment_id")); err != nil {
		s.renderEconomicsError(w, r, "delete equipment economics", err)
		return
	}
	http.Redirect(w, r, "/economics?deleted=1", http.StatusSeeOther)
}

func (s *Server) savePersonnelRate(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	monthlyCost, err := parseRequiredNumber("стоимость персонала в месяц", r.FormValue("monthly_cost"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	availableHours, err := parseRequiredNumber("доступные часы в месяц", r.FormValue("monthly_available_hours"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "load currency for personnel rate", err)
		return
	}
	err = s.lifecycle.SavePersonnelRate(r.Context(), companyIDForRequest(s, r), model.PersonnelRate{
		ID: r.FormValue("id"), Role: r.FormValue("role"),
		MonthlyCost: monthlyCost, MonthlyAvailableHours: availableHours, Currency: settings.Currency,
	})
	if err != nil {
		s.renderEconomicsError(w, r, "save personnel rate", err)
		return
	}
	http.Redirect(w, r, "/economics?saved=1", http.StatusSeeOther)
}

func (s *Server) deletePersonnelRate(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeletePersonnelRate(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		s.renderEconomicsError(w, r, "delete personnel rate", err)
		return
	}
	http.Redirect(w, r, "/economics?deleted=1", http.StatusSeeOther)
}

func (s *Server) saveCompanyOverhead(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	amount, err := parseRequiredNumber("сумма накладных расходов", r.FormValue("amount"))
	if err != nil {
		http.Error(w, applicationPublicError(err), http.StatusBadRequest)
		return
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "load currency for company overhead", err)
		return
	}
	item := model.CompanyOverhead{
		ID: r.FormValue("id"), Name: strings.TrimSpace(r.FormValue("name")),
		Category: r.FormValue("category"), Amount: amount, Period: r.FormValue("period"),
		AllocationMethod: model.AllocationByProductionHours,
		Active:           r.FormValue("active") == "true", Currency: settings.Currency,
	}
	if err := s.lifecycle.SaveCompanyOverhead(r.Context(), companyIDForRequest(s, r), item); err != nil {
		s.renderEconomicsError(w, r, "save company overhead", err)
		return
	}
	http.Redirect(w, r, "/economics?saved=1", http.StatusSeeOther)
}

func (s *Server) deleteCompanyOverhead(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteCompanyOverhead(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		s.renderEconomicsError(w, r, "delete company overhead", err)
		return
	}
	http.Redirect(w, r, "/economics?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderPostgresStockBlanks(w http.ResponseWriter, r *http.Request) {
	companyID := companyIDForRequest(s, r)
	items, err := s.lifecycle.ListStockBlanks(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "list PostgreSQL stock blanks", err)
		return
	}
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "list materials for PostgreSQL stock blanks", err)
		return
	}
	page := s.workspaceBase(r, "Заготовки компании", "stock-blanks", "stock-blanks")
	page.StockBlanks, page.Materials = items, materials
	s.render(w, r, "workspace.html", http.StatusOK, page)
}

func (s *Server) renderEconomicsError(w http.ResponseWriter, r *http.Request, action string, err error) {
	status := http.StatusInternalServerError
	message := "Не удалось сохранить экономические данные. Попробуйте позже."
	switch {
	case errors.Is(err, application.ErrNotFound):
		status, message = http.StatusNotFound, "Запись не найдена или недоступна в текущей компании."
	case isLifecycleValidationError(err):
		status, message = http.StatusBadRequest, applicationPublicError(err)
	default:
		log.Printf("%s: %v", action, err)
	}
	page, loadErr := s.loadEconomicsWorkspace(r)
	if loadErr != nil {
		s.workspaceInternalError(w, "reload economics after error", loadErr)
		return
	}
	page.Error = message
	s.render(w, r, "workspace.html", status, page)
}

func (s *Server) renderPostgresStockBlankForm(w http.ResponseWriter, r *http.Request, create bool) {
	companyID := companyIDForRequest(s, r)
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "load settings for stock blank", err)
		return
	}
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyID)
	if err != nil {
		s.workspaceInternalError(w, "list materials for stock blank form", err)
		return
	}
	page := s.workspaceBase(r, "Новая заготовка", "stock-blanks", "stock-blank-form")
	page.Materials = materials
	page.StockBlankForm = postgresStockBlankForm{Shape: string(model.StockBlankShapeRectangular), Currency: settings.Currency}
	if create {
		s.render(w, r, "workspace.html", http.StatusOK, page)
		return
	}
	if !create {
		items, err := s.lifecycle.ListStockBlanks(r.Context(), companyID)
		if err != nil {
			s.workspaceInternalError(w, "load stock blank for edit", err)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		for _, item := range items {
			if item.ID == id {
				page.Title = "Редактировать заготовку"
				page.StockBlankForm = postgresStockBlankForm{
					ID: item.ID, Name: item.Name, MaterialID: item.MaterialID,
					Shape: string(item.Shape), Length: formatNumber(item.Length),
					Width: formatNumber(item.Width), Height: formatNumber(item.Height),
					Diameter: formatNumber(item.Diameter), PricePerPiece: formatNumber(item.PricePerPiece),
					Currency: item.Currency,
				}
				s.render(w, r, "workspace.html", http.StatusOK, page)
				return
			}
		}
		http.NotFound(w, r)
	}
}

func (s *Server) savePostgresStockBlank(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := postgresStockBlankForm{
		ID: strings.TrimSpace(r.FormValue("id")), Name: strings.TrimSpace(r.FormValue("name")),
		MaterialID: strings.TrimSpace(r.FormValue("material_id")), Shape: strings.TrimSpace(r.FormValue("shape")),
		Length: strings.TrimSpace(r.FormValue("length")), Width: strings.TrimSpace(r.FormValue("width")),
		Height: strings.TrimSpace(r.FormValue("height")), Diameter: strings.TrimSpace(r.FormValue("diameter")),
		PricePerPiece: strings.TrimSpace(r.FormValue("price_per_piece")),
		Currency:      strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	}
	length, err := parseRequiredNumber("Длина", form.Length)
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, err.Error())
		return
	}
	width, err := parseOptionalFloat(form.Width)
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, "Укажите корректную ширину.")
		return
	}
	height, err := parseOptionalFloat(form.Height)
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, "Укажите корректную высоту.")
		return
	}
	diameter, err := parseOptionalFloat(form.Diameter)
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, "Укажите корректный диаметр.")
		return
	}
	price, err := parseRequiredNumber("Цена за штуку", form.PricePerPiece)
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, err.Error())
		return
	}
	_, err = s.lifecycle.SaveStockBlank(r.Context(), companyIDForRequest(s, r), model.StockBlank{
		ID: form.ID, Name: form.Name, MaterialID: form.MaterialID, Shape: model.StockBlankShape(form.Shape),
		Length: length, Width: width, Height: height, Diameter: diameter,
		PricePerPiece: price, Currency: form.Currency,
	})
	if err != nil {
		s.renderPostgresStockBlankError(w, r, form, applicationPublicError(err))
		return
	}
	http.Redirect(w, r, "/stock-blanks?saved=1", http.StatusSeeOther)
}

func (s *Server) renderPostgresStockBlankError(w http.ResponseWriter, r *http.Request, form postgresStockBlankForm, message string) {
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.workspaceInternalError(w, "reload materials for stock blank error", err)
		return
	}
	page := s.workspaceBase(r, "Заготовка", "stock-blanks", "stock-blank-form")
	page.Materials, page.StockBlankForm, page.Error = materials, form, message
	s.render(w, r, "workspace.html", http.StatusBadRequest, page)
}

func (s *Server) deletePostgresStockBlank(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.lifecycle.DeleteStockBlank(r.Context(), companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		s.workspaceInternalError(w, "delete PostgreSQL stock blank", err)
		return
	}
	http.Redirect(w, r, "/stock-blanks?deleted=1", http.StatusSeeOther)
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
		page.ConsumableForm.ResourceMinutes = "0"
		page.ConsumableForm.ResourceUnits = "0"
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
				usedMinutes := make([]string, 0, len(item.UsedMinutesByUnit))
				for _, used := range item.UsedMinutesByUnit {
					usedMinutes = append(usedMinutes, formatNumber(used))
				}
				page.ConsumableForm = consumableForm{
					ID: item.ID, Name: item.Name, Manufacturer: item.Manufacturer, ToolType: item.ToolType,
					DiameterMm: formatNumber(item.DiameterMm), FluteCount: strconv.Itoa(item.FluteCount),
					ToolMaterial: item.ToolMaterial, Coating: item.Coating, LengthMm: formatNumber(item.LengthMm),
					StockQuantity: strconv.Itoa(item.StockQuantity), UnitPrice: formatNumber(item.UnitPrice), Currency: item.Currency,
					ResourceMinutes: formatNumber(item.ResourceMinutes), ResourceUnits: formatNumber(item.ResourceUnits),
					ResourceUnitName: item.ResourceUnitName,
					UsedMinutes:      strings.Join(usedMinutes, " "),
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
		ResourceMinutes:  strings.TrimSpace(r.FormValue("resource_minutes")),
		UsedMinutes:      strings.TrimSpace(r.FormValue("used_minutes")),
		ResourceUnits:    strings.TrimSpace(r.FormValue("resource_units")),
		ResourceUnitName: strings.TrimSpace(r.FormValue("resource_unit_name")),
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
	resourceMinutes, err := parseRequiredNumber("Ресурс инструмента в минутах", form.ResourceMinutes)
	if err != nil {
		s.renderConsumableError(w, r, form, err.Error())
		return
	}
	resourceUnits, err := parseRequiredNumber("Ресурс инструмента в базовых единицах", form.ResourceUnits)
	if err != nil {
		s.renderConsumableError(w, r, form, err.Error())
		return
	}
	var usedMinutes []float64
	for _, line := range strings.Fields(form.UsedMinutes) {
		used, parseErr := parseRequiredNumber("Наработка фрезы", line)
		if parseErr != nil {
			s.renderConsumableError(w, r, form, parseErr.Error())
			return
		}
		usedMinutes = append(usedMinutes, used)
	}
	_, err = s.lifecycle.SaveConsumable(r.Context(), companyIDForRequest(s, r), model.Consumable{
		ID: form.ID, CompanyID: companyIDForRequest(s, r), Category: model.ConsumableCutter,
		Name: form.Name, Manufacturer: form.Manufacturer, ToolType: form.ToolType,
		DiameterMm: diameter, FluteCount: flutes, ToolMaterial: form.ToolMaterial,
		Coating: form.Coating, LengthMm: length, StockQuantity: quantity,
		UnitPrice: price, ResourceMinutes: resourceMinutes, ResourceUnits: resourceUnits,
		UsedMinutesByUnit: usedMinutes,
		ResourceUnitName:  form.ResourceUnitName, Currency: form.Currency, Attributes: json.RawMessage(`{}`),
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

func (s *Server) renderGlobalConsumables(w http.ResponseWriter, r *http.Request) {
	items, err := s.lifecycle.ListGlobalTools(r.Context())
	if err != nil {
		s.workspaceInternalError(w, "list global consumables", err)
		return
	}
	page := s.workspaceBase(r, "Общая база расходников", "catalog-consumables", "catalog-consumables")
	page.Consumables = items
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

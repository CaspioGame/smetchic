package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"smetchic/model"
	"smetchic/storage"
)

const maxFormSize = 1 << 20

type equipmentListPage struct {
	pageBase
	Items []equipmentListRow
}

type equipmentListRow struct {
	Equipment  model.Equipment
	TypeName   string
	StatusName string
}

type equipmentFormPage struct {
	pageBase
	Form   equipmentForm
	Models []model.EquipmentModel
}

type equipmentForm struct {
	ID                 string
	Name               string
	EquipmentModelID   string
	InventoryNumber    string
	SerialNumber       string
	Status             string
	MachineRatePerHour string
}

type materialListPage struct {
	pageBase
	Items []materialListRow
}

type materialListRow struct {
	Material model.Material
	Category string
}

type materialFormPage struct {
	pageBase
	Form materialForm
}

type materialForm struct {
	ID         string
	Name       string
	Grade      string
	Category   string
	Density    string
	PricePerKg string
	Currency   string
}

type stockBlankListPage struct {
	pageBase
	Items []stockBlankRow
}

type stockBlankRow struct {
	Blank    model.StockBlank
	Material string
	Shape    string
	Size     string
	Volume   string
	Mass     string
	Cost     string
	Currency string
	CostNote string
}

type stockBlankFormPage struct {
	pageBase
	Form      stockBlankForm
	Materials []model.Material
}

type stockBlankForm struct {
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

type companyPage struct {
	pageBase
	Company  model.Company
	Settings model.CompanySettings
}

func (s *Server) serveCabinet(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		if r.Method == http.MethodGet {
			s.setCSRFCookie(w, r)
			page := s.defaultIndexPage(r)
			page.Flash = flashMessage(r)
			if len(page.Equipment) == 0 {
				page.Error = "В текущей компании нет доступного оборудования для расчёта."
			}
			s.render(w, r, "index.html", http.StatusOK, page)
			return
		}
		if r.Method == http.MethodPost {
			s.calculate(w, r)
			return
		}
	case "/equipment":
		if r.Method == http.MethodGet {
			s.renderEquipmentList(w, r, "", http.StatusOK)
			return
		}
	case "/equipment/new":
		if r.Method == http.MethodGet {
			s.setCSRFCookie(w, r)
			snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
			if err != nil {
				s.writeInternalError(w, "load equipment form", err)
				return
			}
			s.render(w, r, "equipment-form.html", http.StatusOK, equipmentFormPage{
				pageBase: s.basePage("Новое оборудование", "equipment", "", ""),
				Models:   snapshot.EquipmentModels,
				Form: equipmentForm{
					Status:             string(model.EquipmentStatusActive),
					MachineRatePerHour: "1500",
				},
			})
			return
		}
	case "/equipment/edit":
		if r.Method == http.MethodGet {
			s.renderEquipmentForm(w, r, false)
			return
		}
	case "/equipment/save":
		if r.Method == http.MethodPost {
			s.saveEquipment(w, r)
			return
		}
	case "/equipment/delete":
		if r.Method == http.MethodPost {
			s.deleteEquipment(w, r)
			return
		}
	case "/materials":
		if r.Method == http.MethodGet {
			s.renderMaterialList(w, r, "", http.StatusOK)
			return
		}
	case "/materials/new":
		if r.Method == http.MethodGet {
			s.setCSRFCookie(w, r)
			snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
			if err != nil {
				s.writeInternalError(w, "load material form", err)
				return
			}
			s.render(w, r, "material-form.html", http.StatusOK, materialFormPage{
				pageBase: s.basePage("Новый материал", "materials", "", ""),
				Form:     materialForm{Category: string(model.MaterialCategoryAluminum), Currency: snapshot.Settings.Currency},
			})
			return
		}
	case "/materials/edit":
		if r.Method == http.MethodGet {
			s.renderMaterialForm(w, r, false)
			return
		}
	case "/materials/save":
		if r.Method == http.MethodPost {
			s.saveMaterial(w, r)
			return
		}
	case "/materials/delete":
		if r.Method == http.MethodPost {
			s.deleteMaterial(w, r)
			return
		}
	case "/stock-blanks":
		if r.Method == http.MethodGet {
			s.renderStockBlankList(w, r, "", http.StatusOK)
			return
		}
	case "/stock-blanks/new":
		if r.Method == http.MethodGet {
			s.renderStockBlankForm(w, r, true)
			return
		}
	case "/stock-blanks/edit":
		if r.Method == http.MethodGet {
			s.renderStockBlankForm(w, r, false)
			return
		}
	case "/stock-blanks/save":
		if r.Method == http.MethodPost {
			s.saveStockBlank(w, r)
			return
		}
	case "/stock-blanks/delete":
		if r.Method == http.MethodPost {
			s.deleteStockBlank(w, r)
			return
		}
	case "/company":
		if r.Method == http.MethodGet {
			s.renderCompany(w, r, "", http.StatusOK)
			return
		}
	case "/company/save":
		if r.Method == http.MethodPost {
			s.saveCompany(w, r)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/") {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, r)
	}
}

func (s *Server) renderEquipmentList(w http.ResponseWriter, r *http.Request, message string, status int) {
	s.setCSRFCookie(w, r)
	if message == "" {
		message = flashMessage(r)
	}
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "list equipment", err)
		return
	}
	base := s.basePage("Оборудование", "equipment", message, "")
	if status >= http.StatusBadRequest {
		base.Error, base.Flash = message, ""
	}
	items := make([]equipmentListRow, 0, len(snapshot.Equipment))
	for _, item := range snapshot.Equipment {
		items = append(items, equipmentListRow{
			Equipment: item, TypeName: equipmentTypeLabel(item.EquipmentModel.Type),
			StatusName: equipmentStatusLabel(item.Status),
		})
	}
	s.render(w, r, "equipment.html", status, equipmentListPage{
		pageBase: base,
		Items:    items,
	})
}

func (s *Server) renderEquipmentForm(w http.ResponseWriter, r *http.Request, create bool) {
	s.setCSRFCookie(w, r)
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "load equipment form", err)
		return
	}
	form := equipmentForm{}
	title := "Редактирование оборудования"
	if create {
		title = "Новое оборудование"
		form.Status = string(model.EquipmentStatusActive)
		form.MachineRatePerHour = "1500"
	} else {
		id := r.URL.Query().Get("id")
		item, getErr := s.store.GetEquipment(companyIDForRequest(s, r), id)
		if getErr != nil {
			http.NotFound(w, r)
			return
		}
		form = equipmentForm{
			ID: item.ID, Name: item.Name, EquipmentModelID: item.EquipmentModelID,
			InventoryNumber: item.InventoryNumber, SerialNumber: item.SerialNumber,
			Status: string(item.Status), MachineRatePerHour: formatNumber(item.MachineRatePerHour),
		}
	}
	s.render(w, r, "equipment-form.html", http.StatusOK, equipmentFormPage{
		pageBase: s.basePage(title, "equipment", "", ""),
		Form:     form, Models: snapshot.EquipmentModels,
	})
}

func (s *Server) saveEquipment(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := equipmentForm{
		ID: r.FormValue("id"), Name: strings.TrimSpace(r.FormValue("name")),
		EquipmentModelID:   strings.TrimSpace(r.FormValue("equipment_model_id")),
		InventoryNumber:    strings.TrimSpace(r.FormValue("inventory_number")),
		SerialNumber:       strings.TrimSpace(r.FormValue("serial_number")),
		Status:             strings.TrimSpace(r.FormValue("status")),
		MachineRatePerHour: strings.TrimSpace(r.FormValue("machine_rate_per_hour")),
	}
	rate, err := parseRequiredNumber("Ставка за час", form.MachineRatePerHour)
	if err == nil && rate <= 0 {
		err = fmt.Errorf("Ставка за час должна быть больше нуля.")
	}
	id := form.ID
	if err == nil && id == "" {
		id, err = newRecordID("equipment")
	}
	if err != nil {
		s.renderEquipmentFormError(w, r, form, err)
		return
	}
	item := model.Equipment{
		ID: id, EquipmentModelID: form.EquipmentModelID, Name: form.Name,
		InventoryNumber: form.InventoryNumber, SerialNumber: form.SerialNumber,
		Status: model.EquipmentStatus(form.Status), MachineRatePerHour: rate,
		UpdatedAt: time.Now().UTC(),
	}
	if form.ID != "" {
		old, getErr := s.store.GetEquipment(companyIDForRequest(s, r), form.ID)
		if getErr != nil {
			s.renderEquipmentFormError(w, r, form, storage.ErrNotFound)
			return
		}
		item.CreatedAt = old.CreatedAt
	} else {
		item.CreatedAt = item.UpdatedAt
	}
	if err := s.store.SaveEquipment(companyIDForRequest(s, r), item); err != nil {
		s.renderEquipmentFormError(w, r, form, err)
		return
	}
	http.Redirect(w, r, "/equipment?saved=1", http.StatusSeeOther)
}

func (s *Server) deleteEquipment(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	id := r.FormValue("id")
	if id == "" {
		s.renderEquipmentList(w, r, "Не указан станок для удаления.", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteEquipment(companyIDForRequest(s, r), id); err != nil {
		s.renderEquipmentList(w, r, actionError(err), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/equipment?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderMaterialList(w http.ResponseWriter, r *http.Request, message string, status int) {
	s.setCSRFCookie(w, r)
	if message == "" {
		message = flashMessage(r)
	}
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "list materials", err)
		return
	}
	base := s.basePage("Материалы", "materials", message, "")
	if status >= http.StatusBadRequest {
		base.Error, base.Flash = message, ""
	}
	items := make([]materialListRow, 0, len(snapshot.Materials))
	for _, item := range snapshot.Materials {
		items = append(items, materialListRow{Material: item, Category: materialCategoryLabel(item.Category)})
	}
	s.render(w, r, "materials.html", status, materialListPage{
		pageBase: base,
		Items:    items,
	})
}

func (s *Server) renderMaterialForm(w http.ResponseWriter, r *http.Request, create bool) {
	s.setCSRFCookie(w, r)
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "load material form", err)
		return
	}
	form := materialForm{Currency: snapshot.Settings.Currency}
	title := "Редактирование материала"
	if create {
		title = "Новый материал"
		form.Category = string(model.MaterialCategoryAluminum)
	} else {
		id := r.URL.Query().Get("id")
		var found bool
		for _, item := range snapshot.Materials {
			if item.ID == id {
				form = materialForm{
					ID: item.ID, Name: item.Name, Grade: item.Grade, Category: string(item.Category),
					Density: formatNumber(item.Density), PricePerKg: formatNumber(item.PricePerKg),
					Currency: item.Currency,
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
	s.render(w, r, "material-form.html", http.StatusOK, materialFormPage{
		pageBase: s.basePage(title, "materials", "", ""),
		Form:     form,
	})
}

func (s *Server) saveMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := materialForm{
		ID: r.FormValue("id"), Name: strings.TrimSpace(r.FormValue("name")),
		Grade: strings.TrimSpace(r.FormValue("grade")), Category: r.FormValue("category"),
		Density:    strings.TrimSpace(r.FormValue("density")),
		PricePerKg: strings.TrimSpace(r.FormValue("price_per_kg")),
		Currency:   strings.TrimSpace(r.FormValue("currency")),
	}
	density, err := parseRequiredNumber("Плотность", form.Density)
	if err == nil && density <= 0 {
		err = fmt.Errorf("Плотность должна быть больше нуля.")
	}
	var price float64
	if err == nil {
		price, err = parseRequiredNumber("Цена за кг", form.PricePerKg)
	}
	if err == nil && price < 0 {
		err = fmt.Errorf("Цена за кг не может быть отрицательной.")
	}
	id := form.ID
	if err == nil && id == "" {
		id, err = newRecordID("material")
	}
	if err != nil {
		s.renderMaterialFormError(w, r, form, err)
		return
	}
	item := model.Material{
		ID: id, Name: form.Name, Grade: form.Grade,
		Category: model.MaterialCategory(form.Category),
		Density:  density, PricePerKg: price, Currency: form.Currency,
		UpdatedAt: time.Now().UTC(),
	}
	if form.ID != "" {
		old, findErr := findMaterial(s.store, companyIDForRequest(s, r), form.ID)
		if findErr != nil {
			s.renderMaterialFormError(w, r, form, storage.ErrNotFound)
			return
		}
		item.CreatedAt = old.CreatedAt
	} else {
		item.CreatedAt = item.UpdatedAt
	}
	if err := s.store.SaveMaterial(companyIDForRequest(s, r), item); err != nil {
		s.renderMaterialFormError(w, r, form, err)
		return
	}
	http.Redirect(w, r, "/materials?saved=1", http.StatusSeeOther)
}

func (s *Server) deleteMaterial(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	id := r.FormValue("id")
	if err := s.store.DeleteMaterial(companyIDForRequest(s, r), id); err != nil {
		s.renderMaterialList(w, r, actionError(err), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/materials?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderStockBlankList(w http.ResponseWriter, r *http.Request, message string, status int) {
	s.setCSRFCookie(w, r)
	if message == "" {
		message = flashMessage(r)
	}
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "list stock blanks", err)
		return
	}
	materials := make(map[string]model.Material, len(snapshot.Materials))
	for _, material := range snapshot.Materials {
		materials[material.ID] = material
	}
	rows := make([]stockBlankRow, 0, len(snapshot.StockBlanks))
	for _, blank := range snapshot.StockBlanks {
		row := stockBlankRow{Blank: blank, Currency: blank.Currency}
		material, exists := materials[blank.MaterialID]
		if !exists {
			row.Material = "Материал не найден"
			row.Cost = "Не рассчитано"
			row.CostNote = "Материал отсутствует в текущей компании."
		} else {
			row.Material = material.Name + " (" + material.Grade + ")"
			switch blank.Shape {
			case model.StockBlankShapeRectangular:
				row.Shape = "Прямоугольная"
				row.Size = fmt.Sprintf("%.0f × %.0f × %.0f мм", blank.Length, blank.Width, blank.Height)
			case model.StockBlankShapeRound:
				row.Shape = "Круглая"
				row.Size = fmt.Sprintf("Ø %.0f × %.0f мм", blank.Diameter, blank.Length)
			}
			volume, volumeErr := blank.VolumeMm3()
			mass, massErr := blank.MassKg(material)
			cost, costErr := blank.MaterialCost(material)
			if volumeErr == nil {
				row.Volume = fmt.Sprintf("%.0f мм³", volume)
			}
			if massErr == nil {
				row.Mass = fmt.Sprintf("%.3f кг", mass)
			}
			if costErr == nil {
				row.Cost = fmt.Sprintf("%.2f", cost)
				if blank.PricePerPiece == 0 {
					row.Currency = material.Currency
				}
			} else {
				row.Cost = "Не рассчитано"
				row.CostNote = "Укажите цену за заготовку или цену материала за килограмм в той же валюте."
			}
		}
		rows = append(rows, row)
	}
	base := s.basePage("Заготовки", "blanks", message, "")
	if status >= http.StatusBadRequest {
		base.Error, base.Flash = message, ""
	}
	s.render(w, r, "stock-blanks.html", status, stockBlankListPage{
		pageBase: base,
		Items:    rows,
	})
}

func (s *Server) renderStockBlankForm(w http.ResponseWriter, r *http.Request, create bool) {
	s.setCSRFCookie(w, r)
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "load stock blank form", err)
		return
	}
	form := stockBlankForm{Shape: string(model.StockBlankShapeRectangular), Currency: snapshot.Settings.Currency}
	title := "Новая заготовка"
	if !create {
		title = "Редактирование заготовки"
		id := r.URL.Query().Get("id")
		found := false
		for _, blank := range snapshot.StockBlanks {
			if blank.ID == id {
				form = stockBlankForm{
					ID: blank.ID, Name: blank.Name, MaterialID: blank.MaterialID,
					Shape: string(blank.Shape), Length: formatNumber(blank.Length),
					Width: formatNumber(blank.Width), Height: formatNumber(blank.Height),
					Diameter:      formatNumber(blank.Diameter),
					PricePerPiece: formatNumber(blank.PricePerPiece), Currency: blank.Currency,
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
	s.render(w, r, "stock-blank-form.html", http.StatusOK, stockBlankFormPage{
		pageBase: s.basePage(title, "blanks", "", ""),
		Form:     form, Materials: snapshot.Materials,
	})
}

func (s *Server) saveStockBlank(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	form := stockBlankForm{
		ID: r.FormValue("id"), Name: strings.TrimSpace(r.FormValue("name")),
		MaterialID:    strings.TrimSpace(r.FormValue("material_id")),
		Shape:         strings.TrimSpace(r.FormValue("shape")),
		Length:        strings.TrimSpace(r.FormValue("length")),
		Width:         strings.TrimSpace(r.FormValue("width")),
		Height:        strings.TrimSpace(r.FormValue("height")),
		Diameter:      strings.TrimSpace(r.FormValue("diameter")),
		PricePerPiece: strings.TrimSpace(r.FormValue("price_per_piece")),
		Currency:      strings.TrimSpace(r.FormValue("currency")),
	}
	blank := model.StockBlank{
		ID: form.ID, Name: form.Name, MaterialID: form.MaterialID,
		Shape: model.StockBlankShape(form.Shape), Currency: form.Currency,
		UpdatedAt: time.Now().UTC(),
	}
	var err error
	if blank.Shape == model.StockBlankShapeRectangular {
		blank.Length, err = parseRequiredNumber("Длина", form.Length)
		if err == nil {
			blank.Width, err = parseRequiredNumber("Ширина", form.Width)
		}
		if err == nil {
			blank.Height, err = parseRequiredNumber("Высота", form.Height)
		}
	} else if blank.Shape == model.StockBlankShapeRound {
		blank.Diameter, err = parseRequiredNumber("Диаметр", form.Diameter)
		if err == nil {
			blank.Length, err = parseRequiredNumber("Длина", r.FormValue("length_round"))
		}
	} else {
		err = fmt.Errorf("Выберите допустимую форму заготовки.")
	}
	if err == nil && strings.TrimSpace(form.PricePerPiece) != "" {
		blank.PricePerPiece, err = parseRequiredNumber("Цена за штуку", form.PricePerPiece)
		if err == nil && blank.PricePerPiece < 0 {
			err = fmt.Errorf("Цена за штуку не может быть отрицательной.")
		}
	}
	if err == nil && blank.ID == "" {
		blank.ID, err = newRecordID("blank")
	}
	if err != nil {
		s.renderStockBlankFormError(w, r, form, err)
		return
	}
	if form.ID != "" {
		old, findErr := findStockBlank(s.store, companyIDForRequest(s, r), form.ID)
		if findErr != nil {
			s.renderStockBlankFormError(w, r, form, storage.ErrNotFound)
			return
		}
		blank.CreatedAt = old.CreatedAt
	} else {
		blank.CreatedAt = blank.UpdatedAt
	}
	if err := s.store.SaveStockBlank(companyIDForRequest(s, r), blank); err != nil {
		s.renderStockBlankFormError(w, r, form, err)
		return
	}
	http.Redirect(w, r, "/stock-blanks?saved=1", http.StatusSeeOther)
}

func (s *Server) deleteStockBlank(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	if err := s.store.DeleteStockBlank(companyIDForRequest(s, r), r.FormValue("id")); err != nil {
		s.renderStockBlankList(w, r, actionError(err), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/stock-blanks?deleted=1", http.StatusSeeOther)
}

func (s *Server) renderCompany(w http.ResponseWriter, r *http.Request, message string, status int) {
	s.setCSRFCookie(w, r)
	if message == "" {
		message = flashMessage(r)
	}
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "load company settings", err)
		return
	}
	base := s.basePage("Компания и настройки", "company", message, "")
	if status >= http.StatusBadRequest {
		base.Error, base.Flash = message, ""
	}
	s.render(w, r, "company.html", status, companyPage{
		pageBase: base,
		Company:  snapshot.Company, Settings: snapshot.Settings,
	})
}

func (s *Server) saveCompany(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	currency := strings.TrimSpace(r.FormValue("currency"))
	markupText := strings.TrimSpace(r.FormValue("default_markup_percent"))
	markup, err := parseRequiredNumber("Наценка по умолчанию", markupText)
	if err == nil && markup < 0 {
		err = fmt.Errorf("Наценка не может быть отрицательной.")
	}
	if err != nil {
		s.renderCompany(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	snapshot, err := s.store.Snapshot(companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "load company before save", err)
		return
	}
	snapshot.Company.Name = name
	snapshot.Company.UpdatedAt = time.Now().UTC()
	snapshot.Settings.Currency = currency
	snapshot.Settings.DefaultMarkupPercent = markup
	if err := s.store.SaveCompany(snapshot.Company, snapshot.Settings); err != nil {
		s.renderCompany(w, r, friendlyError(err), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/company?saved=1", http.StatusSeeOther)
}

func (s *Server) renderEquipmentFormError(w http.ResponseWriter, r *http.Request, form equipmentForm, err error) {
	s.setCSRFCookie(w, r)
	snapshot, loadErr := s.store.Snapshot(companyIDForRequest(s, r))
	if loadErr != nil {
		s.writeInternalError(w, "reload equipment form", loadErr)
		return
	}
	s.render(w, r, "equipment-form.html", http.StatusBadRequest, equipmentFormPage{
		pageBase: s.basePage("Оборудование", "equipment", "", friendlyError(err)),
		Form:     form, Models: snapshot.EquipmentModels,
	})
}

func (s *Server) renderMaterialFormError(w http.ResponseWriter, r *http.Request, form materialForm, err error) {
	s.setCSRFCookie(w, r)
	s.render(w, r, "material-form.html", http.StatusBadRequest, materialFormPage{
		pageBase: s.basePage("Материал", "materials", "", friendlyError(err)),
		Form:     form,
	})
}

func (s *Server) renderStockBlankFormError(w http.ResponseWriter, r *http.Request, form stockBlankForm, err error) {
	s.setCSRFCookie(w, r)
	snapshot, loadErr := s.store.Snapshot(companyIDForRequest(s, r))
	if loadErr != nil {
		s.writeInternalError(w, "reload stock blank form", loadErr)
		return
	}
	s.render(w, r, "stock-blank-form.html", http.StatusBadRequest, stockBlankFormPage{
		pageBase: s.basePage("Заготовка", "blanks", "", friendlyError(err)),
		Form:     form, Materials: snapshot.Materials,
	})
}

func (s *Server) parseMutationForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Некорректные данные формы.", http.StatusBadRequest)
		return false
	}
	session, ok := serverSession{}, false
	if s.authentication != nil {
		session, ok = s.sessionFromRequest(r)
	}
	if s.authentication != nil && (!ok || !s.validCSRF(r, session)) {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу и повторите попытку.", http.StatusForbidden)
		return false
	}
	cookie, err := r.Cookie("cnc_csrf")
	expected := s.csrfToken
	if s.authentication != nil {
		expected = session.CSRFToken
	}
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) != 1 ||
		subtle.ConstantTimeCompare([]byte(r.FormValue("csrf_token")), []byte(expected)) != 1 {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу и повторите попытку.", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) setCSRFCookie(w http.ResponseWriter, r *http.Request) {
	token := s.csrfToken
	expires := time.Time{}
	maxAge := 0
	if s.authentication != nil {
		session, ok := s.sessionFromRequest(r)
		if !ok {
			return
		}
		token = session.CSRFToken
		expires = session.ExpiresAt
		maxAge = int(time.Until(session.ExpiresAt).Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: s.secureCookie(r), SameSite: http.SameSiteStrictMode,
		Expires: expires, MaxAge: maxAge,
	})
}

func (s *Server) basePage(title, active, flash, message string) pageBase {
	base := pageBase{
		Title: title, Active: active, CSRF: s.csrfToken,
		Error: message, LifecycleEnabled: s.lifecycle != nil,
	}
	switch flash {
	case "saved":
		base.Flash = "Изменения сохранены."
	case "deleted":
		base.Flash = "Запись удалена."
	default:
		base.Flash = flash
	}
	return base
}

func flashMessage(r *http.Request) string {
	switch r.URL.Query().Get("saved") {
	case "1":
		return "Изменения сохранены."
	}
	if r.URL.Query().Get("deleted") == "1" {
		return "Запись удалена."
	}
	return ""
}

func newRecordID(prefix string) (string, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("не удалось создать идентификатор записи")
	}
	return prefix + "-" + hex.EncodeToString(random), nil
}

func actionError(err error) string {
	switch {
	case errors.Is(err, storage.ErrReferenced):
		return "Нельзя удалить материал: он используется одной или несколькими заготовками."
	case errors.Is(err, storage.ErrNotFound):
		return "Запись не найдена или недоступна в текущей компании."
	case errors.Is(err, storage.ErrCompanyMismatch):
		return "Операция недоступна для данных другой компании."
	default:
		log.Printf("операция со справочником не выполнена: %v", err)
		return "Не удалось выполнить операцию. Проверьте данные и повторите попытку."
	}
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	switch {
	case errors.Is(err, storage.ErrReferenced):
		return "Нельзя удалить материал: он используется одной или несколькими заготовками."
	case errors.Is(err, storage.ErrNotFound):
		return "Запись не найдена или недоступна в текущей компании."
	case errors.Is(err, storage.ErrCompanyMismatch):
		return "Выберите данные, принадлежащие текущей компании."
	case strings.HasPrefix(text, "Заполните поле"),
		strings.Contains(text, "корректное число"),
		strings.Contains(text, "должна быть"),
		strings.Contains(text, "не может быть"):
		return text
	case strings.Contains(text, "оборудование с ID"):
		return text
	case strings.Contains(text, "company name"):
		return "Укажите название компании."
	case strings.Contains(text, "material name"):
		return "Укажите название материала."
	case strings.Contains(text, "material grade"):
		return "Укажите марку материала."
	case strings.Contains(text, "stock blank name"):
		return "Укажите название заготовки."
	case strings.Contains(text, "unsupported stock blank shape"):
		return "Выберите форму заготовки: прямоугольная или круглая."
	case strings.Contains(text, "density"):
		return "Плотность материала должна быть больше нуля."
	case strings.Contains(text, "price per kg"):
		return "Цена за килограмм должна быть неотрицательным числом."
	case strings.Contains(text, "currency"):
		return "Укажите валюту цены."
	case strings.Contains(text, "category"):
		return "Выберите допустимую категорию материала."
	case strings.Contains(text, "stock blank"):
		return "Проверьте размеры и обязательные поля заготовки."
	case strings.Contains(text, "equipment model"):
		return "Выберите модель оборудования из списка."
	case strings.Contains(text, "equipment"):
		return "Проверьте данные оборудования."
	case strings.Contains(text, "material"):
		return "Проверьте данные материала и его принадлежность компании."
	case strings.Contains(text, "same company"):
		return "Материал и заготовка должны принадлежать одной компании."
	default:
		log.Printf("ошибка изменения справочника: %v", err)
		return "Не удалось сохранить данные. Проверьте заполнение полей."
	}
}

func (s *Server) writeInternalError(w http.ResponseWriter, action string, err error) {
	log.Printf("%s: %v", action, err)
	http.Error(w, "Не удалось загрузить данные. Попробуйте обновить страницу.", http.StatusInternalServerError)
}

func findMaterial(store *storage.Store, companyID, id string) (model.Material, error) {
	snapshot, err := store.Snapshot(companyID)
	if err != nil {
		return model.Material{}, err
	}
	for _, item := range snapshot.Materials {
		if item.ID == id {
			return item, nil
		}
	}
	return model.Material{}, storage.ErrNotFound
}

func findStockBlank(store *storage.Store, companyID, id string) (model.StockBlank, error) {
	snapshot, err := store.Snapshot(companyID)
	if err != nil {
		return model.StockBlank{}, err
	}
	for _, item := range snapshot.StockBlanks {
		if item.ID == id {
			return item, nil
		}
	}
	return model.StockBlank{}, storage.ErrNotFound
}

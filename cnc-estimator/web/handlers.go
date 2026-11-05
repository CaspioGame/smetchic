package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"smetchic/estimator"
	"smetchic/model"
	"smetchic/parser"
	"smetchic/storage"
)

const (
	maxUploadSize  = 2 << 20
	memoryFormSize = 1 << 20
)

type pageBase struct {
	Title            string
	Active           string
	CSRF             string
	Flash            string
	Error            string
	LifecycleEnabled bool
}

type indexPage struct {
	pageBase
	SelectedEquipment string
	MarkupPercent     string
	Equipment         []model.Equipment
}

type resultPage struct {
	pageBase
	Estimate          model.Estimate
	EquipmentTypeName string
	Features          []featureRow
	Operations        []operationRow
}

type featureRow struct {
	ID          string
	Type        string
	Dimensions  string
	Coordinates string
}

type operationRow struct {
	FeatureName string
	Operation   model.Operation
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.lifecycle != nil && s.serveLifecycle(w, r) {
		return
	}
	if s.store != nil {
		s.serveCabinet(w, r)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.render(w, "index.html", http.StatusOK, s.defaultIndexPage())
	case http.MethodPost:
		s.calculate(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
	}
}

func (s *Server) calculate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(memoryFormSize); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			s.renderError(w, http.StatusRequestEntityTooLarge, "Файл слишком большой. Максимальный размер — 2 МБ.")
			return
		}
		s.renderError(w, http.StatusBadRequest, "Не удалось прочитать загруженный файл. Проверьте multipart-форму.")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	form := indexPage{
		pageBase: pageBase{
			Title: "Расчёт сметы", Active: "estimate", CSRF: s.csrfToken,
			LifecycleEnabled: s.lifecycle != nil,
		},
		SelectedEquipment: r.FormValue("equipment_id"),
		MarkupPercent:     r.FormValue("markup_percent"),
		Equipment:         s.equipmentList(),
	}
	equipmentItem, settings, err := s.parseEstimateSettings(form)
	if err != nil {
		form.Error = friendlyError(err)
		s.render(w, "index.html", http.StatusBadRequest, form)
		return
	}

	file, header, err := r.FormFile("part_file")
	if err != nil {
		form.Error = "Выберите файл .part для расчёта."
		s.render(w, "index.html", http.StatusBadRequest, form)
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." {
		form.Error = "Имя загруженного файла некорректно."
		s.render(w, "index.html", http.StatusBadRequest, form)
		return
	}

	part, err := parser.Parse(file, filename)
	if err != nil {
		form.Error = fmt.Sprintf("Не удалось обработать файл %s: %v", filename, err)
		s.render(w, "index.html", http.StatusBadRequest, form)
		return
	}
	estimate, err := estimator.Calculate(part, equipmentItem, settings)
	if err != nil {
		form.Error = fmt.Sprintf("Не удалось рассчитать файл %s: %v", filename, err)
		s.render(w, "index.html", http.StatusBadRequest, form)
		return
	}

	page := buildResultPage(estimate)
	page.LifecycleEnabled = s.lifecycle != nil
	s.render(w, "result.html", http.StatusOK, page)
}

func (s *Server) defaultIndexPage() indexPage {
	equipmentItems := s.equipmentList()
	equipmentID := s.defaultSettings.EquipmentID
	markup := s.defaultSettings.MarkupPercent
	if s.store != nil {
		if snapshot, err := s.store.Snapshot(s.companyID); err == nil {
			markup = snapshot.Settings.DefaultMarkupPercent
			for _, item := range equipmentItems {
				if item.Status == model.EquipmentStatusActive {
					equipmentID = item.ID
					break
				}
			}
		}
	}
	return indexPage{
		pageBase:          pageBase{Title: "Расчёт сметы", Active: "estimate", CSRF: s.csrfToken, LifecycleEnabled: s.lifecycle != nil},
		SelectedEquipment: equipmentID,
		MarkupPercent:     formatNumber(markup),
		Equipment:         equipmentItems,
	}
}

func (s *Server) renderError(w http.ResponseWriter, status int, message string) {
	page := s.defaultIndexPage()
	page.Error = message
	s.render(w, "index.html", status, page)
}

func (s *Server) parseEstimateSettings(form indexPage) (model.Equipment, model.EstimateSettings, error) {
	equipmentItem, err := s.catalog.GetByID(form.SelectedEquipment)
	if s.store != nil {
		equipmentItem, err = s.store.GetEquipment(s.companyID, form.SelectedEquipment)
		if err != nil && errors.Is(err, storage.ErrNotFound) {
			return model.Equipment{}, model.EstimateSettings{}, fmt.Errorf("оборудование с ID %q не найдено", form.SelectedEquipment)
		}
	}
	if err != nil {
		return model.Equipment{}, model.EstimateSettings{}, err
	}
	markup, err := parseRequiredNumber("Наценка", form.MarkupPercent)
	if err != nil {
		return model.Equipment{}, model.EstimateSettings{}, err
	}
	if markup < 0 {
		return model.Equipment{}, model.EstimateSettings{}, fmt.Errorf("Наценка не может быть отрицательной.")
	}
	return equipmentItem, model.EstimateSettings{
		EquipmentID:   equipmentItem.ID,
		MarkupPercent: markup,
	}, nil
}

func parseRequiredNumber(label, value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("Заполните поле «%s».", label)
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("Поле «%s» должно содержать корректное число.", label)
	}
	return number, nil
}

func formatNumber(number float64) string {
	return strconv.FormatFloat(number, 'f', -1, 64)
}

func equipmentTypeLabel(equipmentType model.EquipmentType) string {
	labels := map[model.EquipmentType]string{
		model.EquipmentMilling:   "Фрезерный станок",
		model.EquipmentTurning:   "Токарный станок",
		model.EquipmentLaser:     "Лазерное оборудование",
		model.Equipment3DPrinter: "3D-принтер",
		model.EquipmentWelding:   "Сварочное оборудование",
		model.EquipmentPress:     "Пресс",
		model.EquipmentOther:     "Другое оборудование",
	}
	if label, exists := labels[equipmentType]; exists {
		return label
	}
	return string(equipmentType)
}

func equipmentStatusLabel(status model.EquipmentStatus) string {
	switch status {
	case model.EquipmentStatusActive:
		return "Активен"
	case model.EquipmentStatusMaintenance:
		return "На обслуживании"
	case model.EquipmentStatusInactive:
		return "Неактивен"
	default:
		return string(status)
	}
}

func materialCategoryLabel(category model.MaterialCategory) string {
	labels := map[model.MaterialCategory]string{
		model.MaterialCategoryAluminum:       "Алюминий",
		model.MaterialCategorySteel:          "Сталь",
		model.MaterialCategoryStainlessSteel: "Нержавеющая сталь",
		model.MaterialCategoryBrass:          "Латунь",
		model.MaterialCategoryCopper:         "Медь",
		model.MaterialCategoryPlastic:        "Пластик",
		model.MaterialCategoryOther:          "Другое",
	}
	if label, exists := labels[category]; exists {
		return label
	}
	return string(category)
}

func buildResultPage(estimate model.Estimate) resultPage {
	features := make([]featureRow, 0, len(estimate.Part.Features))
	featureNames := make(map[string]string, len(estimate.Part.Features))
	counts := make(map[model.FeatureType]int)
	for _, feature := range estimate.Part.Features {
		counts[feature.Type]++
		name := featureDisplayName(feature.Type, counts[feature.Type])
		featureNames[feature.ID] = name
		features = append(features, featureRow{
			ID:          feature.ID,
			Type:        string(feature.Type),
			Dimensions:  describeFeature(feature),
			Coordinates: describeCoordinates(feature),
		})
	}

	operations := make([]operationRow, 0, len(estimate.Operations))
	for _, operation := range estimate.Operations {
		operations = append(operations, operationRow{
			FeatureName: featureNames[operation.FeatureID],
			Operation:   operation,
		})
	}
	return resultPage{
		pageBase:          pageBase{Title: "Результат расчёта", Active: "estimate"},
		Estimate:          estimate,
		EquipmentTypeName: equipmentTypeLabel(estimate.Equipment.EquipmentModel.Type),
		Features:          features,
		Operations:        operations,
	}
}

func (s *Server) equipmentList() []model.Equipment {
	if s.store != nil {
		snapshot, err := s.store.Snapshot(s.companyID)
		if err != nil {
			return nil
		}
		return snapshot.Equipment
	}
	return s.catalog.List()
}

func featureDisplayName(featureType model.FeatureType, index int) string {
	names := map[model.FeatureType]string{
		model.FeaturePocket:  "Pocket",
		model.FeatureHole:    "Hole",
		model.FeatureContour: "Contour",
		model.FeatureChamfer: "Chamfer",
	}
	name := names[featureType]
	if featureType == model.FeatureContour && index == 1 {
		return name
	}
	return fmt.Sprintf("%s #%d", name, index)
}

func describeFeature(feature model.Feature) string {
	switch feature.Type {
	case model.FeaturePocket, model.FeatureContour:
		return fmt.Sprintf("%.0f × %.0f × %.0f mm", feature.Width, feature.Length, feature.Depth)
	case model.FeatureHole:
		return fmt.Sprintf("Ø%.0f × %.0f mm", feature.Diameter, feature.Depth)
	case model.FeatureChamfer:
		return fmt.Sprintf("%.1f mm chamfer", feature.ChamferSize)
	default:
		return ""
	}
}

func describeCoordinates(feature model.Feature) string {
	if math.IsNaN(feature.PositionX) || math.IsNaN(feature.PositionY) ||
		(feature.PositionX == 0 && feature.PositionY == 0) {
		return ""
	}
	return fmt.Sprintf("X %.2f, Y %.2f mm", feature.PositionX, feature.PositionY)
}

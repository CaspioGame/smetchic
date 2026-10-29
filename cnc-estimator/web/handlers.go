package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"

	"smetchic/estimator"
	"smetchic/model"
	"smetchic/parser"
)

const (
	maxUploadSize  = 2 << 20
	memoryFormSize = 1 << 20
)

type indexPage struct {
	Error string
}

type resultPage struct {
	Estimate   model.Estimate
	Features   []featureRow
	Operations []operationRow
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
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.render(w, "index.html", http.StatusOK, indexPage{})
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

	file, header, err := r.FormFile("part_file")
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "Выберите файл .part для расчёта.")
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." {
		s.renderError(w, http.StatusBadRequest, "Имя загруженного файла некорректно.")
		return
	}

	part, err := parser.Parse(file, filename)
	if err != nil {
		s.renderError(w, http.StatusBadRequest, fmt.Sprintf("Не удалось обработать файл %s: %v", filename, err))
		return
	}
	estimate, err := estimator.Calculate(part, s.machine)
	if err != nil {
		s.renderError(w, http.StatusBadRequest, fmt.Sprintf("Не удалось рассчитать файл %s: %v", filename, err))
		return
	}

	s.render(w, "result.html", http.StatusOK, buildResultPage(estimate))
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
	return resultPage{Estimate: estimate, Features: features, Operations: operations}
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

package parser

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"smetchic/model"
)

type rawFeature struct {
	id     string
	line   int
	values map[string]string
}

func ParseFile(filename string) (model.Part, error) {
	file, err := os.Open(filename)
	if err != nil {
		return model.Part{}, fmt.Errorf("не удалось открыть файл %q: %w", filename, err)
	}
	defer file.Close()

	return Parse(file, filename)
}

func Parse(reader io.Reader, sourceName string) (model.Part, error) {
	filename := sourceName
	part := model.Part{SourceFile: filepath.Base(filename)}
	partValues := make(map[string]string)
	partValueLines := make(map[string]int)
	features := make([]rawFeature, 0)
	var currentFeature *rawFeature
	currentSection := ""
	partSectionSeen := false
	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if currentFeature != nil {
				features = append(features, *currentFeature)
				currentFeature = nil
			}
			section := strings.TrimSpace(line[1 : len(line)-1])
			if section == "part" {
				if partSectionSeen {
					return model.Part{}, fmt.Errorf("%s:%d: секция [part] указана повторно", filename, lineNumber)
				}
				partSectionSeen = true
				currentSection = "part"
				continue
			}
			if strings.HasPrefix(section, "feature ") {
				featureID := strings.TrimSpace(strings.TrimPrefix(section, "feature "))
				if featureID == "" || strings.ContainsAny(featureID, " \t") {
					return model.Part{}, fmt.Errorf("%s:%d: ожидается секция [feature id]", filename, lineNumber)
				}
				currentFeature = &rawFeature{
					id:     featureID,
					line:   lineNumber,
					values: make(map[string]string),
				}
				currentSection = "feature"
				continue
			}
			return model.Part{}, fmt.Errorf("%s:%d: неизвестная секция %q", filename, lineNumber, section)
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return model.Part{}, fmt.Errorf("%s:%d: ожидается параметр key=value или секция", filename, lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return model.Part{}, fmt.Errorf("%s:%d: ключ и значение не должны быть пустыми", filename, lineNumber)
		}

		var values map[string]string
		switch currentSection {
		case "part":
			values = partValues
			partValueLines[key] = lineNumber
		case "feature":
			values = currentFeature.values
		default:
			return model.Part{}, fmt.Errorf("%s:%d: параметр указан вне секции", filename, lineNumber)
		}
		if _, exists := values[key]; exists {
			return model.Part{}, fmt.Errorf("%s:%d: параметр %q указан повторно", filename, lineNumber, key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return model.Part{}, fmt.Errorf("не удалось прочитать файл %q: %w", filename, err)
	}
	if currentFeature != nil {
		features = append(features, *currentFeature)
	}

	if !partSectionSeen {
		return model.Part{}, fmt.Errorf("%s: не указана секция [part]", filename)
	}

	allowedPartKeys := map[string]bool{
		"material": true, "stock_width": true, "stock_length": true, "stock_height": true,
	}
	for key := range partValues {
		if !allowedPartKeys[key] {
			return model.Part{}, fmt.Errorf("%s:%d: неизвестный параметр детали %q", filename, partValueLines[key], key)
		}
	}
	part.Material = partValues["material"]
	if part.Material == "" {
		return model.Part{}, fmt.Errorf("%s: не указан material", filename)
	}
	var err error
	part.Stock.Width, err = requiredDimension(filename, partValues, partValueLines, "stock_width")
	if err != nil {
		return model.Part{}, err
	}
	part.Stock.Length, err = requiredDimension(filename, partValues, partValueLines, "stock_length")
	if err != nil {
		return model.Part{}, err
	}
	part.Stock.Height, err = requiredDimension(filename, partValues, partValueLines, "stock_height")
	if err != nil {
		return model.Part{}, err
	}

	seenFeatureIDs := make(map[string]bool)
	for _, raw := range features {
		if seenFeatureIDs[raw.id] {
			return model.Part{}, fmt.Errorf("%s:%d: feature id %q указан повторно", filename, raw.line, raw.id)
		}
		seenFeatureIDs[raw.id] = true
		feature, err := parseFeature(filename, raw)
		if err != nil {
			return model.Part{}, err
		}
		part.Features = append(part.Features, feature)
	}
	if len(part.Features) == 0 {
		return model.Part{}, fmt.Errorf("%s: деталь должна содержать хотя бы один feature", filename)
	}
	return part, nil
}

func requiredDimension(filename string, values map[string]string, lines map[string]int, key string) (float64, error) {
	value, exists := values[key]
	if !exists {
		return 0, fmt.Errorf("%s: не указан %s", filename, key)
	}
	return parseDimension(filename, lines[key], key, value)
}

func parseFeature(filename string, raw rawFeature) (model.Feature, error) {
	feature := model.Feature{ID: raw.id, Type: model.FeatureType(strings.ToLower(raw.values["type"]))}
	allowedKeys := map[string]bool{"type": true, "tolerance": true}
	requiredKeys := []string{}
	switch feature.Type {
	case model.FeaturePocket:
		requiredKeys = []string{"width", "length", "depth"}
		allowedKeys["x"], allowedKeys["y"] = true, true
	case model.FeatureHole:
		requiredKeys = []string{"diameter", "depth"}
		allowedKeys["x"], allowedKeys["y"] = true, true
	case model.FeatureContour:
		requiredKeys = []string{"width", "length", "depth"}
	case model.FeatureChamfer:
		requiredKeys = []string{"chamfer_size"}
	default:
		return model.Feature{}, fmt.Errorf("%s:%d: неизвестный тип feature %q", filename, raw.line, feature.Type)
	}
	for key := range raw.values {
		if !allowedKeys[key] && key != "width" && key != "length" && key != "depth" && key != "diameter" && key != "chamfer_size" {
			return model.Feature{}, fmt.Errorf("%s:%d: неизвестный параметр feature %q", filename, raw.line, key)
		}
		if !allowedKeys[key] {
			isRequiredForType := false
			for _, requiredKey := range requiredKeys {
				if key == requiredKey {
					isRequiredForType = true
					break
				}
			}
			if !isRequiredForType {
				return model.Feature{}, fmt.Errorf("%s:%d: параметр %q не подходит для feature типа %q", filename, raw.line, key, feature.Type)
			}
		}
	}
	if _, exists := raw.values["type"]; !exists {
		return model.Feature{}, fmt.Errorf("%s:%d: не указан type feature", filename, raw.line)
	}
	for _, key := range requiredKeys {
		if _, exists := raw.values[key]; !exists {
			return model.Feature{}, fmt.Errorf("%s:%d: для feature %q не указан %s", filename, raw.line, raw.id, key)
		}
	}

	for _, dimension := range []struct {
		key    string
		target *float64
	}{
		{"width", &feature.Width},
		{"length", &feature.Length},
		{"depth", &feature.Depth},
		{"diameter", &feature.Diameter},
		{"chamfer_size", &feature.ChamferSize},
	} {
		value, exists := raw.values[dimension.key]
		if !exists {
			continue
		}
		parsed, err := parseDimension(filename, raw.line, dimension.key, value)
		if err != nil {
			return model.Feature{}, err
		}
		*dimension.target = parsed
	}
	for _, position := range []struct {
		key    string
		target *float64
	}{
		{"x", &feature.PositionX},
		{"y", &feature.PositionY},
	} {
		value, exists := raw.values[position.key]
		if exists {
			parsed, err := parseCoordinate(filename, raw.line, position.key, value)
			if err != nil {
				return model.Feature{}, err
			}
			*position.target = parsed
		}
	}
	_, hasX := raw.values["x"]
	_, hasY := raw.values["y"]
	if hasX != hasY {
		return model.Feature{}, fmt.Errorf("%s:%d: position требует оба параметра x и y", filename, raw.line)
	}
	if value, exists := raw.values["tolerance"]; exists {
		tolerance, err := parseCoordinate(filename, raw.line, "tolerance", value)
		if err != nil {
			return model.Feature{}, err
		}
		feature.Tolerance = tolerance
	}
	return feature, nil
}

func parseDimension(filename string, lineNumber int, key, value string) (float64, error) {
	dimension, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(dimension) || math.IsInf(dimension, 0) || dimension <= 0 {
		return 0, fmt.Errorf("%s: %s должен быть положительным числом в миллиметрах", inputLocation(filename, lineNumber), key)
	}
	return dimension, nil
}

func parseCoordinate(filename string, lineNumber int, key, value string) (float64, error) {
	coordinate, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(coordinate) || math.IsInf(coordinate, 0) || coordinate < 0 {
		return 0, fmt.Errorf("%s: %s должен быть неотрицательным числом в миллиметрах", inputLocation(filename, lineNumber), key)
	}
	return coordinate, nil
}

func inputLocation(filename string, lineNumber int) string {
	if lineNumber == 0 {
		return filename
	}
	return fmt.Sprintf("%s:%d", filename, lineNumber)
}

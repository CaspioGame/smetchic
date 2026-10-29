package planner

import (
	"fmt"
	"math"
	"strings"

	"smetchic/model"
)

func BuildOperations(part model.Part, machine model.Machine) ([]model.Operation, error) {
	operations := make([]model.Operation, 0)
	addOperation := func(feature model.Feature, name, strategy string, tool model.Tool, parameters model.CuttingParameters, testLength float64) {
		operations = append(operations, model.Operation{
			ID:                      fmt.Sprintf("operation-%02d", len(operations)+1),
			FeatureID:               feature.ID,
			Name:                    name,
			Strategy:                strategy,
			Machine:                 machine,
			Tool:                    tool,
			CuttingParameters:       parameters,
			EstimatedToolpathLength: testLength,
		})
	}

	endMill := model.Tool{Name: "End mill", Diameter: 10, MaxRPM: 6000}
	drill := model.Tool{Name: "Drill", Diameter: 10, MaxRPM: 5000}
	chamferTool := model.Tool{Name: "Chamfer mill", Diameter: 6, MaxRPM: 5000}

	for _, feature := range part.Features {
		if err := validateFeature(feature); err != nil {
			return nil, err
		}

		switch feature.Type {
		case model.FeaturePocket:
			addOperation(feature, "Roughing", "Roughing", endMill, model.CuttingParameters{
				Feed: 1000, RPM: 3000, Depth: feature.Depth, StepDown: 5, StepOver: 5,
			}, 600)
			addOperation(feature, "Finishing", "Finishing", endMill, model.CuttingParameters{
				Feed: 800, RPM: 3000, Depth: feature.Depth, StepDown: 1, StepOver: 1,
			}, 240)
		case model.FeatureHole:
			holeTool := drill
			holeTool.Diameter = feature.Diameter
			addOperation(feature, "Drilling", "Drilling", holeTool, model.CuttingParameters{
				Feed: 500, RPM: 2500, Depth: feature.Depth,
			}, 60)
		case model.FeatureContour:
			addOperation(feature, "Roughing", "Roughing", endMill, model.CuttingParameters{
				Feed: 900, RPM: 3000, Depth: feature.Depth, StepDown: 3, StepOver: 5,
			}, 500)
			addOperation(feature, "Finishing", "Finishing", endMill, model.CuttingParameters{
				Feed: 700, RPM: 3000, Depth: feature.Depth, StepDown: 1, StepOver: 1,
			}, 200)
		case model.FeatureChamfer:
			addOperation(feature, "Chamfering", "Chamfering", chamferTool, model.CuttingParameters{
				Feed: 600, RPM: 2500, Depth: feature.ChamferSize,
			}, 120)
		default:
			return nil, fmt.Errorf("planner не поддерживает feature %q типа %q", feature.ID, feature.Type)
		}
	}
	return operations, nil
}

func validateFeature(feature model.Feature) error {
	if strings.TrimSpace(feature.ID) == "" {
		return fmt.Errorf("planner: у feature типа %q не указан ID", feature.Type)
	}

	var requiredDimensions []struct {
		name  string
		value float64
	}
	switch feature.Type {
	case model.FeaturePocket:
		requiredDimensions = []struct {
			name  string
			value float64
		}{{"width", feature.Width}, {"length", feature.Length}, {"depth", feature.Depth}}
	case model.FeatureHole:
		requiredDimensions = []struct {
			name  string
			value float64
		}{{"diameter", feature.Diameter}, {"depth", feature.Depth}}
	case model.FeatureContour:
		requiredDimensions = []struct {
			name  string
			value float64
		}{{"width", feature.Width}, {"length", feature.Length}, {"depth", feature.Depth}}
	case model.FeatureChamfer:
		requiredDimensions = []struct {
			name  string
			value float64
		}{{"chamfer_size", feature.ChamferSize}}
	default:
		return fmt.Errorf("planner: неизвестный тип feature %q (ID %q)", feature.Type, feature.ID)
	}

	for _, dimension := range requiredDimensions {
		if math.IsNaN(dimension.value) || math.IsInf(dimension.value, 0) || dimension.value <= 0 {
			return fmt.Errorf("planner: feature %q типа %q требует %s больше нуля", feature.ID, feature.Type, dimension.name)
		}
	}
	for _, optional := range []struct {
		name  string
		value float64
	}{
		{"x", feature.PositionX},
		{"y", feature.PositionY},
		{"tolerance", feature.Tolerance},
	} {
		if math.IsNaN(optional.value) || math.IsInf(optional.value, 0) || optional.value < 0 {
			return fmt.Errorf("planner: feature %q имеет некорректный параметр %s", feature.ID, optional.name)
		}
	}
	return nil
}

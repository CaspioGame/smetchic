package planner

import (
	"testing"

	"smetchic/model"
)

func TestBuildOperationsValidatesFeatures(t *testing.T) {
	tests := []struct {
		name    string
		feature model.Feature
	}{
		{
			name:    "unknown type",
			feature: model.Feature{ID: "unknown", Type: model.FeatureType("unknown")},
		},
		{
			name:    "pocket without depth",
			feature: model.Feature{ID: "pocket", Type: model.FeaturePocket, Width: 10, Length: 10},
		},
		{
			name:    "hole without diameter",
			feature: model.Feature{ID: "hole", Type: model.FeatureHole, Depth: 10},
		},
		{
			name:    "contour without dimensions",
			feature: model.Feature{ID: "contour", Type: model.FeatureContour, Depth: 1},
		},
		{
			name:    "chamfer without size",
			feature: model.Feature{ID: "chamfer", Type: model.FeatureChamfer},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildOperations(model.Part{Features: []model.Feature{test.feature}}, model.Machine{})
			if err == nil {
				t.Fatal("BuildOperations() expected an error")
			}
		})
	}
}

func TestBuildOperationsLeavesTimeCalculationToCalculator(t *testing.T) {
	part := model.Part{Features: []model.Feature{
		{ID: "pocket", Type: model.FeaturePocket, Width: 40, Length: 40, Depth: 20},
		{ID: "hole", Type: model.FeatureHole, Diameter: 10, Depth: 30},
		{ID: "contour", Type: model.FeatureContour, Width: 120, Length: 100, Depth: 2},
		{ID: "chamfer", Type: model.FeatureChamfer, ChamferSize: 1},
	}}

	operations, err := BuildOperations(part, model.Machine{Name: "test machine"})
	if err != nil {
		t.Fatalf("BuildOperations() error = %v", err)
	}
	if len(operations) != 6 {
		t.Fatalf("BuildOperations() returned %d operations, want 6", len(operations))
	}
	for _, operation := range operations {
		if operation.TimeMinutes != 0 {
			t.Errorf("operation %q has TimeMinutes %v before time calculation", operation.ID, operation.TimeMinutes)
		}
	}
}

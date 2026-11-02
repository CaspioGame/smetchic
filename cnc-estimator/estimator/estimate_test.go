package estimator

import (
	"testing"

	"smetchic/model"
)

func TestCalculateUsesEquipmentForOperationsAndCost(t *testing.T) {
	part := model.Part{Features: []model.Feature{
		{ID: "hole-1", Type: model.FeatureHole, Diameter: 10, Depth: 30},
	}}
	equipment := model.Equipment{
		ID:                 "test-mill",
		CompanyID:          "test-company",
		EquipmentModelID:   "test-model",
		Name:               "Test Mill",
		Status:             model.EquipmentStatusActive,
		MachineRatePerHour: 3000,
		EquipmentModel: model.EquipmentModel{
			ID:      "test-model",
			Type:    model.EquipmentMilling,
			MaxFeed: 400,
		},
	}
	settings := model.EstimateSettings{EquipmentID: equipment.ID, MarkupPercent: 50}

	estimate, err := Calculate(part, equipment, settings)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if estimate.Equipment.ID != equipment.ID {
		t.Errorf("estimate equipment ID = %q, want %q", estimate.Equipment.ID, equipment.ID)
	}
	if len(estimate.Operations) != 1 {
		t.Fatalf("operation count = %d, want 1", len(estimate.Operations))
	}
	if estimate.Operations[0].Equipment.ID != equipment.ID {
		t.Errorf("operation equipment ID = %q, want %q", estimate.Operations[0].Equipment.ID, equipment.ID)
	}
	if estimate.Operations[0].CuttingParameters.Feed != 400 {
		t.Errorf("operation feed = %v, want equipment max feed 400", estimate.Operations[0].CuttingParameters.Feed)
	}
	if estimate.MachineCost != 7.5 {
		t.Errorf("machine cost = %v, want 7.5 using equipment rate", estimate.MachineCost)
	}
	if estimate.SuggestedPrice != 11.25 {
		t.Errorf("suggested price = %v, want 11.25", estimate.SuggestedPrice)
	}
}

func TestCalculateRejectsEquipmentSettingsMismatch(t *testing.T) {
	_, err := Calculate(model.Part{}, model.Equipment{ID: "dekart-s4040"}, model.EstimateSettings{EquipmentID: "unknown"})
	if err == nil {
		t.Fatal("Calculate() expected an error for mismatched equipment ID")
	}
}

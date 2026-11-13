package application

import "testing"

func TestValidateEstimateInputRequiresReasonsForManualValues(t *testing.T) {
	tests := []struct {
		name    string
		input   EstimateInput
		wantErr bool
	}{
		{
			name: "manual setup without reason",
			input: EstimateInput{
				Quantity: 1, EquipmentID: "equipment", SetupTimeMinutes: 10,
				MarkupSource: EstimateSourceCompanyData,
			},
			wantErr: true,
		},
		{
			name: "manual tool usage without reason",
			input: EstimateInput{
				Quantity: 1, EquipmentID: "equipment", ToolUsageMinutes: 5,
				MarkupSource: EstimateSourceCompanyData,
			},
			wantErr: true,
		},
		{
			name: "manual markup without reason",
			input: EstimateInput{
				Quantity: 1, EquipmentID: "equipment", MarkupPercent: 10,
				MarkupSource: EstimateSourceUserOverride,
			},
			wantErr: true,
		},
		{
			name: "manual setup with reason",
			input: EstimateInput{
				Quantity: 1, EquipmentID: "equipment", SetupTimeMinutes: 10,
				SetupSource: EstimateSourceUserOverride, SetupReason: "Fixture change",
				MarkupSource: EstimateSourceCompanyData,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalizeEstimateSources(&test.input)
			err := validateEstimateInput(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateEstimateInput() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

package costing

import (
	"math"
	"testing"

	"smetchic/model"
)

func TestCalculateProductionCostBreakdown(t *testing.T) {
	operator := model.PersonnelRate{MonthlyCost: 3000, MonthlyAvailableHours: 150}
	programmer := model.PersonnelRate{MonthlyCost: 3000, MonthlyAvailableHours: 150}
	breakdown, err := Calculate(Input{
		Quantity: 2, Currency: "USD", MachineTimeMinutes: 60,
		EquipmentEconomics: &model.EquipmentEconomics{
			CompanyID: "company", EquipmentID: "machine", PurchasePrice: 11000,
			ResidualValue: 1000, UsefulLifeYears: 10, AnnualAvailableHours: 1000,
			ConsumptionKW: 2, ElectricityPricePerKWh: 0.5,
			AnnualMaintenanceCost: 1000, AnnualRepairCost: 500, AnnualOtherCost: 500,
		},
		OperatorRate: &operator, ProgrammingRate: &programmer, ProgrammingHours: 2,
		SetupTimeMinutes: 30, SetupCostPerHour: 60,
		MaterialCostPerPiece: 10, MaterialPriceSource: "company_stock_blank",
		Tools: []ToolUse{{Name: "End mill", PurchasePrice: 100, ResourceMinutes: 1000, UsedMinutes: 10}},
		CompanyEconomics: model.CompanyEconomics{
			CompanyID: "company", DefaultMarkupPercent: 10, MonthlyRent: 600,
			RentAllocationMethod:   model.AllocationByProductionHours,
			MonthlyProductionHours: 120, Currency: "USD",
		},
		Overheads: []model.CompanyOverhead{
			{
				ID: "utilities", CompanyID: "company", Name: "Utilities",
				Category: model.OverheadUtilities, Amount: 240, Period: model.OverheadMonthly,
				AllocationMethod: model.AllocationByProductionHours, Active: true, Currency: "USD",
			},
			{
				ID: "rent", CompanyID: "company", Name: "Annual overhead",
				Category: model.OverheadOther, Amount: 1200, Period: model.OverheadAnnual,
				AllocationMethod: model.AllocationByProductionHours, Active: true, Currency: "USD",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "depreciation", breakdown.DepreciationCost, 2)
	assertNear(t, "electricity", breakdown.ElectricityCost, 2)
	assertNear(t, "machine", breakdown.MachineCost, 8)
	assertNear(t, "operator", breakdown.OperatorCost, 40)
	assertNear(t, "programming", breakdown.ProgrammingCost, 40)
	assertNear(t, "tooling", breakdown.ToolingCost, 2)
	assertNear(t, "setup", breakdown.SetupCost, 30)
	assertNear(t, "rent", breakdown.RentCost, 10)
	assertNear(t, "utilities", breakdown.UtilitiesCost, 4)
	assertNear(t, "other overhead", breakdown.OtherOverheadCost, 5.0/3)
	assertNear(t, "batch", breakdown.BatchCost, 70)
	assertNear(t, "per piece", breakdown.PerPieceCost, 257.0/6)
	assertNear(t, "total", breakdown.TotalCost, 467.0/3)
	assertNear(t, "selling price", breakdown.SellingPrice, 513.7/3)
	assertNear(t, "per-piece selling price", breakdown.SellingPricePerPiece, 513.7/6)
}

func TestCalculateUsesExplicitLegacyRateAndDoesNotInventCosts(t *testing.T) {
	breakdown, err := Calculate(Input{
		Quantity: 3, Currency: "RUB", MachineTimeMinutes: 30,
		LegacyMachineRatePerHour: 120,
		CompanyEconomics: model.CompanyEconomics{
			CompanyID: "company", RentAllocationMethod: model.AllocationByProductionHours, Currency: "RUB",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "legacy machine cost", breakdown.MachineCost, 180)
	assertNear(t, "cost total", breakdown.TotalCost, 180)
	assertNear(t, "markup", breakdown.SellingPrice, 180)
	if len(breakdown.Warnings) == 0 {
		t.Fatal("expected warnings for unconfigured economy inputs")
	}
}

func TestCalculateRejectsInvalidQuantityAndNonFiniteInputs(t *testing.T) {
	base := Input{
		Quantity: 1, Currency: "RUB",
		CompanyEconomics: model.CompanyEconomics{
			CompanyID: "company", RentAllocationMethod: model.AllocationByProductionHours, Currency: "RUB",
		},
	}
	base.Quantity = 0
	if _, err := Calculate(base); err == nil {
		t.Fatal("zero quantity was accepted")
	}
	base.Quantity = 1
	base.MachineTimeMinutes = math.NaN()
	if _, err := Calculate(base); err == nil {
		t.Fatal("NaN machine time was accepted")
	}
}

func assertNear(t *testing.T, name string, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 0.00001 {
		t.Errorf("%s = %.6f, want %.6f", name, actual, expected)
	}
}

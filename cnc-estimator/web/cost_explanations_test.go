package web

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"smetchic/application"
	"smetchic/costing"
	"smetchic/model"
)

func TestCostExplanationsUseSnapshotForEveryCostArticle(t *testing.T) {
	company := model.CompanyEconomics{CompanyID: "company", MonthlyRent: 20000, MonthlyProductionHours: 160,
		Currency: "RUB", RentAllocationMethod: model.AllocationByProductionHours, DefaultMarkupPercent: 50}
	equipment := model.EquipmentEconomics{CompanyID: "company", EquipmentID: "machine", PurchasePrice: 300000,
		UsefulLifeYears: 5, AnnualAvailableHours: 1500, ConsumptionKW: 2, ElectricityPricePerKWh: 10, AnnualMaintenanceCost: 30000}
	rates := []model.PersonnelRate{
		{Role: "operator", MonthlyCost: 80000, MonthlyAvailableHours: 160},
		{Role: "setup", MonthlyCost: 96000, MonthlyAvailableHours: 160},
		{Role: "programmer", MonthlyCost: 160000, MonthlyAvailableHours: 160},
	}
	overheads := []model.CompanyOverhead{
		{ID: "water", CompanyID: "company", Name: "Вода", Category: model.OverheadUtilities, Amount: 1200, Period: model.OverheadMonthly, AllocationMethod: model.AllocationByProductionHours, Active: true, Currency: "RUB"},
		{ID: "heating", CompanyID: "company", Name: "Отопление", Category: model.OverheadUtilities, Amount: 12000, Period: model.OverheadAnnual, AllocationMethod: model.AllocationByProductionHours, Active: true, Currency: "RUB"},
		{ID: "other", CompanyID: "company", Name: "Связь", Category: model.OverheadOther, Amount: 600, Period: model.OverheadMonthly, AllocationMethod: model.AllocationByProductionHours, Active: true, Currency: "RUB"},
	}
	cost, err := costing.Calculate(costing.Input{Quantity: 43, Currency: "RUB", MachineTimeMinutes: 30,
		EquipmentEconomics: &equipment, CompanyEconomics: company, OperatorRate: &rates[0], ProgrammingRate: &rates[2],
		ProgrammingHours: 5, SetupTimeMinutes: 60, SetupCostPerHour: 600, MaterialCostPerPiece: 2250,
		Tools: []costing.ToolUse{{PurchasePrice: 1000, ResourceMinutes: 120, UsedMinutes: 30}}, Overheads: overheads})
	if err != nil {
		t.Fatal(err)
	}
	settings := savedCostSettings{CompanyEconomics: &company, EquipmentEconomics: &equipment, PersonnelRates: rates, Overheads: overheads,
		StockBlank: &model.StockBlank{PricePerPiece: 2250}, Consumable: &model.Consumable{UnitPrice: 1000, ResourceMinutes: 120}}
	settingsData, _ := json.Marshal(settings)
	meta := estimateVersionSnapshot{WorkingInput: application.EstimateInput{Name: "Смета", ToolUsageMinutes: 30, SetupTimeMinutes: 60, ProgrammingHours: 5}, UsedMachineTimeHours: 0.5}
	inputData, _ := json.Marshal(meta)
	originalSettings := append([]byte(nil), settingsData...)
	originalInput := append([]byte(nil), inputData...)
	page := lifecyclePage{Result: model.Estimate{CostBreakdown: &cost}, View: "version"}
	if err := prepareEstimateCostDisplay(&page, settingsData, inputData); err != nil {
		t.Fatal(err)
	}
	for _, item := range page.CostItems {
		explanation := page.CostCalculations[item.Name]
		if len(explanation.Batch) == 0 || explanation.Note != "" || explanation.PerPiece.Formula == "" {
			t.Errorf("no full explanation for %s: %+v", item.Name, explanation)
		}
	}
	if got := page.CostCalculations["Материал"].Batch[0].Values; got != "2 250.00 ₽ × 43 = 96 750.00 ₽" {
		t.Fatalf("material: %q", got)
	}
	if got := page.CostCalculations["Программирование"].Batch[0].Values; !strings.Contains(got, "5 ч = 5 000.00 ₽") {
		t.Fatalf("programming: %q", got)
	}
	if !bytes.Equal(settingsData, originalSettings) || !bytes.Equal(inputData, originalInput) {
		t.Fatal("explanation modified source snapshots")
	}
	server, err := newServer(nil, nil, "", model.EstimateSettings{})
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"estimate", "version"} {
		page.View = view
		var rendered bytes.Buffer
		if err := server.templates.ExecuteTemplate(&rendered, "lifecycle.html", page); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(rendered.String(), "<details class=\"cost-details\">") ||
			!strings.Contains(rendered.String(), "96 750.00 ₽") || !strings.Contains(rendered.String(), "5 ч = 5 000.00 ₽") {
			t.Fatalf("missing interactive explanation in %s", view)
		}
	}
}

func TestCostExplanationsFixedFeesAndIncompleteHistoricalSnapshots(t *testing.T) {
	fixed := 1000.0
	b := model.ProductionCostBreakdown{Quantity: 20, Currency: "RUB", SetupCost: 1000, OperatorCost: 0, MaterialCost: 200}
	meta := estimateVersionSnapshot{WorkingInput: application.EstimateInput{
		Name: "История", Modules: application.EstimateModules{application.ModuleSetup}, ProjectLabor: &model.ProjectLabor{Setup: &fixed},
	}}
	inputData, _ := json.Marshal(meta)
	page := lifecyclePage{Result: model.Estimate{CostBreakdown: &b}}
	if err := prepareEstimateCostDisplay(&page, []byte("{}"), inputData); err != nil {
		t.Fatal(err)
	}
	if got := page.CostCalculations["Наладка"]; got.Batch[0].Values != "1 000.00 ₽ = 1 000.00 ₽" ||
		got.PerPiece.Values != "1 000.00 ₽ ÷ 20 = 50.00 ₽" {
		t.Fatalf("fixed fee: %+v", got)
	}
	if len(page.CostCalculations["Оператор"].Batch) == 0 {
		t.Fatal("missing disabled operator explanation")
	}
	if err := prepareEstimateCostDisplay(&page, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if len(page.CostCalculations["Наладка"].Batch) != 0 || page.CostCalculations["Наладка"].Note == "" {
		t.Fatal("fabricated missing historical rate")
	}
	if len(page.CostCalculations["Материал"].Batch) == 0 {
		t.Fatal("saved material amounts should still be explained")
	}
}

func TestCostExplanationsRejectMismatchingSavedRate(t *testing.T) {
	page := lifecyclePage{Result: model.Estimate{CostBreakdown: &model.ProductionCostBreakdown{
		Quantity: 1, Currency: "RUB", EquipmentEconomicsApplied: true, ElectricityCost: 10,
	}}}
	// A sparse legacy snapshot has incompatible inputs. Do not display a false equality.
	settings := []byte(`{"equipment_economics":{"consumption_kw":999,"electricity_price_per_kwh":10}}`)
	input := []byte(`{"used_machine_time_hours":1}`)
	if err := prepareEstimateCostDisplay(&page, settings, input); err != nil {
		t.Fatal(err)
	}
	if got := page.CostCalculations["Электроэнергия станка"]; len(got.Batch) != 0 || got.Note == "" {
		t.Fatalf("mismatching rate was presented as the calculation: %+v", got)
	}
}

func TestCostExplanationsMaterialByWeightAndLegacyHourlyRate(t *testing.T) {
	page := lifecyclePage{Result: model.Estimate{CostBreakdown: &model.ProductionCostBreakdown{
		Quantity: 2, Currency: "RUB", MaterialCost: 1000, MachineCost: 600,
	}}}
	settings := []byte(`{"global_material":{"density":2700,"reference_price_kg":200},"equipment":{"machine_rate_per_hour":1200}}`)
	input := []byte(`{"working_input":{"name":"История","material_weight_kg":2.5},"used_machine_time_hours":0.25}`)
	if err := prepareEstimateCostDisplay(&page, settings, input); err != nil {
		t.Fatal(err)
	}
	if got := page.CostCalculations["Материал"].Batch[0].Values; got != "2.5 кг × 200.00 ₽/кг × 2 = 1 000.00 ₽" {
		t.Fatalf("weight cost: %q", got)
	}
	machine := page.CostCalculations["Затраты на станок"]
	if len(machine.Batch) != 2 || !strings.Contains(machine.Batch[1].Values, "1 200.00 ₽/ч × 0.25 ч × 2 = 600.00 ₽") {
		t.Fatalf("legacy machine rate explanation: %+v", machine)
	}
	// The saved material density gives 0.1 × 0.1 × 0.1 m³ × 2700 kg/m³ = 2.7 kg.
	page.Result.Part.Stock = model.Stock{Width: 100, Length: 100, Height: 100}
	page.Result.CostBreakdown.MaterialCost = 1080
	if err := prepareEstimateCostDisplay(&page, settings, []byte(`{"used_machine_time_hours":0.25}`)); err != nil {
		t.Fatal(err)
	}
	material := page.CostCalculations["Материал"]
	if len(material.Batch) != 2 || !strings.Contains(material.Batch[0].Values, "= 2.7 кг") ||
		material.Batch[1].Values != "2.7 кг × 200.00 ₽/кг × 2 = 1 080.00 ₽" {
		t.Fatalf("geometry material explanation: %+v", material)
	}
}

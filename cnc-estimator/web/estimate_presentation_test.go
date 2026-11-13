package web

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"smetchic/application"
	"smetchic/model"
)

func TestEstimateTemplatesRenderWithoutCostBreakdown(t *testing.T) {
	server, err := newServer(nil, nil, "", model.EstimateSettings{})
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"estimate", "version"} {
		t.Run(view, func(t *testing.T) {
			page := lifecyclePage{
				pageBase: pageBase{Title: "Test", LifecycleEnabled: true},
				View:     view,
			}
			var body bytes.Buffer
			if err := server.templates.ExecuteTemplate(&body, "lifecycle.html", page); err != nil {
				t.Fatalf("render %s view: %v", view, err)
			}
			for _, hiddenLabel := range []string{
				"Снимок входных данных", "Снимок настроек", "Полный результат расчёта",
			} {
				if strings.Contains(body.String(), hiddenLabel) {
					t.Errorf("%s view contains hidden technical section %q", view, hiddenLabel)
				}
			}
		})
	}
}

func TestHistoricalEstimateResultSnapshotDisplaysSavedCostBreakdown(t *testing.T) {
	result, err := decodeEstimateResult([]byte(`{
		"CostBreakdown": {
			"Quantity": 20,
			"Currency": "RUB",
			"MaterialCost": 39936,
			"MaterialPriceSource": "global_reference_price",
			"EquipmentEconomicsApplied": true,
			"DepreciationCost": 0,
			"ElectricityCost": 19.21,
			"MaintenanceCost": 19.21,
			"RepairCost": 0,
			"OtherEquipmentCost": 6.4,
			"MachineCost": 44.82,
			"OperatorCost": 582.07,
			"ProgrammingCost": 0,
			"ToolingCost": 0,
			"SetupCost": 0,
			"RentCost": 116.41,
			"UtilitiesCost": 0,
			"OtherOverheadCost": 0,
			"TotalCost": 40679.31,
			"TotalPerPieceCost": 2033.9655,
			"Profit": 20339.655,
			"MarkupPercent": 50,
			"SellingPrice": 61018.965,
			"SellingPricePerPiece": 3050.94825,
			"Warnings": [
				"Global material price is a reference estimate, not a supplier quotation.",
				"Tooling is not selected; tooling cost is excluded.",
				"Programming rate is not configured; programming cost is excluded."
			]
		}
	}`))
	if err != nil {
		t.Fatalf("decode legacy result snapshot: %v", err)
	}

	var page lifecyclePage
	page.Result = result
	settingsSnapshot := []byte(`{
		"GlobalMaterial": {"Name": "Алюминий", "Grade": "6061"},
		"CompanySettings": {"DefaultMarkupPercent": 3}
	}`)
	if err := prepareEstimateCostDisplay(&page, settingsSnapshot); err != nil {
		t.Fatalf("prepare historical cost display: %v", err)
	}
	if page.MaterialName != "Алюминий 6061" {
		t.Fatalf("material name = %q, want saved snapshot material", page.MaterialName)
	}
	if page.MaterialPriceLabel != "Ориентировочная цена из общей базы материалов" {
		t.Fatalf("material source = %q", page.MaterialPriceLabel)
	}
	if len(page.CostItems) == 0 || page.CostItems[0].Name != "Материал" ||
		page.CostItems[0].BatchCost != 39936 || page.CostItems[0].PerPieceCost != 1996.8 {
		t.Fatalf("legacy cost breakdown = %#v", page.CostItems)
	}

	server, err := newServer(nil, nil, "", model.EstimateSettings{})
	if err != nil {
		t.Fatal(err)
	}
	page.pageBase = pageBase{Title: "Историческая версия", LifecycleEnabled: true}
	page.View = "version"
	var body bytes.Buffer
	if err := server.templates.ExecuteTemplate(&body, "lifecycle.html", page); err != nil {
		t.Fatalf("render historical version: %v", err)
	}
	rendered := body.String()
	for _, expected := range []string{
		"Алюминий 6061", "Ориентировочная цена из общей базы материалов",
		"Электроэнергия станка", "Обслуживание станка", "Прочие расходы станка",
		"Оператор", "Аренда", "40 679.31 ₽", "2 033.97 ₽", "3 050.95 ₽",
		"Цена материала взята из общей базы и является ориентировочной",
		"Инструмент не выбран", "Не настроена стоимость программирования",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered historical version does not contain %q", expected)
		}
	}
	for _, internal := range []string{"global_reference_price", "programming", "tooling"} {
		if strings.Contains(rendered, internal) {
			t.Errorf("rendered historical version exposes internal identifier %q", internal)
		}
	}
	if strings.Contains(rendered, "Подробная разбивка расходов не сохранена") {
		t.Error("rendered historical version claims saved cost breakdown is missing")
	}
}

func TestHistoricalEstimateResultSnapshotWithRootCostBreakdown(t *testing.T) {
	result, err := decodeEstimateResult([]byte(`{
		"Quantity": 5, "Currency": "RUB", "MaterialCost": 120,
		"MaterialPriceSource": "global_reference_price", "TotalCost": 175,
		"TotalPerPieceCost": 35, "OperatorCost": 55
	}`))
	if err != nil {
		t.Fatalf("decode root cost breakdown snapshot: %v", err)
	}
	if result.CostBreakdown == nil || result.CostBreakdown.MaterialCost != 120 || result.CostBreakdown.TotalCost != 175 {
		t.Fatalf("saved root breakdown = %#v", result.CostBreakdown)
	}
	page := lifecyclePage{Result: result}
	if err := prepareEstimateCostDisplay(&page, nil); err != nil {
		t.Fatal(err)
	}
	if len(page.CostItems) == 0 || page.CostItems[0].BatchCost != 120 {
		t.Fatalf("display rows = %#v, want saved material cost", page.CostItems)
	}
}

func TestHistoricalEstimateSnapshotWithCostItemsUsesSavedRows(t *testing.T) {
	result, err := decodeEstimateResult([]byte(`{
		"cost_breakdown": {
			"quantity": 2,
			"currency": "RUB",
			"material_cost": 100,
			"material_price_source": "company_stock_blank",
			"total_cost": 150,
			"total_per_piece_cost": 75,
			"cost_items": [
				{"name": "Материал", "batch_cost": 100, "per_piece_cost": 50},
				{"name": "Себестоимость", "batch_cost": 150, "per_piece_cost": 75},
				{"name": "Прибыль", "batch_cost": 15, "per_piece_cost": 7.5}
			]
		}
	}`))
	if err != nil {
		t.Fatalf("decode result snapshot with saved rows: %v", err)
	}
	page := lifecyclePage{Result: result}
	if err := prepareEstimateCostDisplay(&page, []byte(`{"Material":{"Name":"Сталь","Grade":"45"}}`)); err != nil {
		t.Fatalf("prepare saved cost rows: %v", err)
	}
	if len(page.CostItems) < 9 || page.CostItems[0].BatchCost != 100 {
		t.Fatalf("cost items = %#v, want saved and legacy detail rows", page.CostItems)
	}
	for _, item := range page.CostItems {
		if item.Name == "Себестоимость" || item.Name == "Прибыль" || item.Name == "Цена продажи" {
			t.Errorf("summary row %q is duplicated in the detail table", item.Name)
		}
	}
	if page.MaterialName != "Сталь 45" {
		t.Fatalf("material name = %q, want saved settings snapshot name", page.MaterialName)
	}
}

func TestFormatMoneyUsesGroupedAmountAndCurrency(t *testing.T) {
	tests := []struct {
		amount   float64
		currency string
		want     string
	}{
		{1234567.8, "RUB", "1 234 567.80 ₽"},
		{-1234.5, "USD", "-1 234.50 $"},
		{12, "XYZ", "12.00 XYZ"},
	}
	for _, test := range tests {
		if got := formatMoney(test.amount, test.currency); got != test.want {
			t.Errorf("formatMoney(%v, %q) = %q, want %q", test.amount, test.currency, got, test.want)
		}
	}
}

func TestEstimateInputFormValuesPreserveWorkingParameters(t *testing.T) {
	input := application.EstimateInput{
		Name: "Batch 42", OrderID: "order-7", Quantity: 12, EquipmentID: "machine-3",
		MarkupPercent: 18.5, MarkupSource: application.EstimateSourceUserOverride,
		MarkupReason: "Customer agreement", StockBlankID: "blank-2", MaterialID: "material-4",
		GlobalMaterialID: "global-1", ConsumableID: "tool-5",
		ToolUsageMinutes: 1.25, ToolUsageReason: "Measured use",
		SetupTimeMinutes: 8, SetupReason: "Fixture change",
		ProgrammingHours: 0.5, ProgrammingReason: "New program",
		MachineTime: &application.EstimateOverride{
			Value: 4.5 / 60, Source: application.EstimateSourceUserOverride, Reason: "Tested cycle",
		},
	}
	values := estimateInputFormValues(input, 3.25, 4.5, true)
	want := map[string]string{
		"name": "Batch 42", "order_id": "order-7", "quantity": "12", "equipment_id": "machine-3",
		"machine_time_hours": "4.50", "markup_percent": "18.5", "markup_source": "user_override",
		"markup_reason": "Customer agreement", "stock_blank_id": "blank-2", "material_id": "material-4",
		"global_material_id": "global-1", "consumable_id": "tool-5",
		"tool_usage_minutes": "1.25", "tool_usage_reason": "Measured use",
		"setup_time_minutes": "8", "setup_reason": "Fixture change",
		"programming_hours": "0.5", "programming_reason": "New program",
	}
	for key, expected := range want {
		if values[key] != expected {
			t.Errorf("form value %q = %q, want %q", key, values[key], expected)
		}
	}
}

func TestCompareEstimateVersionsShowsChangedValuesAndReason(t *testing.T) {
	makeSnapshot := func(quantity int, usedHours float64, reason string) json.RawMessage {
		data, err := json.Marshal(estimateVersionSnapshot{
			WorkingInput:         application.EstimateInput{Name: "Estimate", Quantity: quantity, MarkupPercent: 10},
			UsedMachineTimeHours: usedHours, MachineTimeReason: reason,
		})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	previous := application.EstimateVersionRecord{
		VersionNumber: 1, InputSnapshot: makeSnapshot(5, 0.1, ""), ResultSnapshot: []byte("{}"),
		MarkupPercent: 10,
	}
	current := application.EstimateVersionRecord{
		VersionNumber: 2, InputSnapshot: makeSnapshot(8, 0.15, "Test cut"), ResultSnapshot: []byte("{}"),
		MarkupPercent: 12,
	}
	changes, err := compareEstimateVersions(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3: %#v", len(changes), changes)
	}
	if changes[0].Name != "Количество" || changes[0].Before != "5" || changes[0].After != "8" {
		t.Errorf("quantity change = %#v", changes[0])
	}
	if changes[1].Name != "Машинное время на деталь" || changes[1].Before != "6.00 мин" ||
		changes[1].After != "9.00 мин" || changes[1].Reason != "Test cut" {
		t.Errorf("time change = %#v", changes[1])
	}
	if changes[2].Name != "Наценка" || changes[2].Before != "10.00%" || changes[2].After != "12.00%" {
		t.Errorf("markup change = %#v", changes[2])
	}
}

func TestNormalizeEstimateResultForDisplayBackfillsLegacyPerPieceTotal(t *testing.T) {
	result := model.Estimate{
		CostBreakdown: &model.ProductionCostBreakdown{Quantity: 4, TotalCost: 125},
	}
	normalizeEstimateResultForDisplay(&result)
	if result.CostBreakdown.TotalPerPieceCost != 31.25 {
		t.Fatalf("legacy total per piece = %v, want 31.25", result.CostBreakdown.TotalPerPieceCost)
	}
}

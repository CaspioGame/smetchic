package model

import (
	"math"
	"strings"
	"testing"
)

func testMaterial() Material {
	return Material{
		ID: "aluminum-amg6", CompanyID: "company-1", Name: "Алюминий АМг6",
		Grade: "АМг6", Category: MaterialCategoryAluminum,
		Density: 2700, PricePerKg: 100, Currency: "RUB",
	}
}

func testRectangularBlank() StockBlank {
	return StockBlank{
		ID: "blank-rect-1", CompanyID: "company-1", MaterialID: "aluminum-amg6",
		Name: "Плита 100 × 50 × 20", Shape: StockBlankShapeRectangular,
		Length: 100, Width: 50, Height: 20, Currency: "RUB",
	}
}

func TestMaterialValidation(t *testing.T) {
	material := testMaterial()
	if err := material.Validate(); err != nil {
		t.Fatalf("valid material rejected: %v", err)
	}

	tests := []struct {
		name    string
		change  func(*Material)
		wantErr string
	}{
		{
			name:    "zero density",
			change:  func(material *Material) { material.Density = 0 },
			wantErr: "density",
		},
		{
			name:    "negative price",
			change:  func(material *Material) { material.PricePerKg = -1 },
			wantErr: "price per kg",
		},
		{
			name:    "non-finite density",
			change:  func(material *Material) { material.Density = math.Inf(1) },
			wantErr: "density",
		},
		{
			name:    "unknown category",
			change:  func(material *Material) { material.Category = MaterialCategory("wood") },
			wantErr: "category",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := testMaterial()
			test.change(&invalid)
			assertMaterialError(t, invalid.Validate(), test.wantErr)
		})
	}
}

func TestRectangularStockBlankVolumeAndMass(t *testing.T) {
	blank := testRectangularBlank()
	volume, err := blank.VolumeMm3()
	if err != nil {
		t.Fatalf("VolumeMm3() error = %v", err)
	}
	if volume != 100_000 {
		t.Errorf("volume = %v mm³, want 100000 mm³", volume)
	}

	mass, err := blank.MassKg(testMaterial())
	if err != nil {
		t.Fatalf("MassKg() error = %v", err)
	}
	if math.Abs(mass-0.27) > 1e-12 {
		t.Errorf("mass = %v kg, want 0.27 kg", mass)
	}
}

func TestRoundStockBlankVolumeAndMass(t *testing.T) {
	blank := StockBlank{
		ID: "blank-round-1", CompanyID: "company-1", MaterialID: "steel-45",
		Name: "Круглый пруток Ø20 × 100", Shape: StockBlankShapeRound,
		Diameter: 20, Length: 100, Currency: "RUB",
	}
	material := Material{
		ID: "steel-45", CompanyID: "company-1", Name: "Сталь 45",
		Grade: "45", Category: MaterialCategorySteel,
		Density: 8000, PricePerKg: 0, Currency: "RUB",
	}

	volume, err := blank.VolumeMm3()
	if err != nil {
		t.Fatalf("VolumeMm3() error = %v", err)
	}
	wantVolume := math.Pi * 10 * 10 * 100
	if math.Abs(volume-wantVolume) > 1e-10 {
		t.Errorf("volume = %v mm³, want %v mm³", volume, wantVolume)
	}

	mass, err := blank.MassKg(material)
	if err != nil {
		t.Fatalf("MassKg() error = %v", err)
	}
	wantMass := wantVolume * material.Density / 1_000_000_000
	if math.Abs(mass-wantMass) > 1e-12 {
		t.Errorf("mass = %v kg, want %v kg", mass, wantMass)
	}
}

func TestStockBlankMaterialCostUsesPricePerPiece(t *testing.T) {
	blank := testRectangularBlank()
	blank.PricePerPiece = 42.5

	cost, err := blank.MaterialCost(testMaterial())
	if err != nil {
		t.Fatalf("MaterialCost() error = %v", err)
	}
	if cost != 42.5 {
		t.Errorf("material cost = %v, want piece price 42.5", cost)
	}
}

func TestStockBlankMaterialCostUsesMassAndPricePerKg(t *testing.T) {
	cost, err := testRectangularBlank().MaterialCost(testMaterial())
	if err != nil {
		t.Fatalf("MaterialCost() error = %v", err)
	}
	if math.Abs(cost-27) > 1e-12 {
		t.Errorf("material cost = %v, want 27", cost)
	}
}

func TestStockBlankMaterialCostWithoutPricesReturnsError(t *testing.T) {
	material := testMaterial()
	material.PricePerKg = 0
	_, err := testRectangularBlank().MaterialCost(material)
	assertMaterialError(t, err, "price per kg is unavailable")
}

func TestStockBlankMaterialCostRejectsDifferentCurrencies(t *testing.T) {
	material := testMaterial()
	material.Currency = "EUR"
	_, err := testRectangularBlank().MaterialCost(material)
	assertMaterialError(t, err, "currencies must match")
}

func TestStockBlankRejectsInvalidDimensions(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*StockBlank)
		wantErr string
	}{
		{
			name:    "rectangular width zero",
			change:  func(blank *StockBlank) { blank.Width = 0 },
			wantErr: "width",
		},
		{
			name: "round missing diameter",
			change: func(blank *StockBlank) {
				blank.Shape = StockBlankShapeRound
				blank.Width = 0
				blank.Height = 0
				blank.Diameter = 0
			},
			wantErr: "diameter",
		},
		{
			name: "rectangular does not require diameter",
			change: func(blank *StockBlank) {
				blank.Diameter = 0
			},
		},
		{
			name: "round does not require width or height",
			change: func(blank *StockBlank) {
				blank.Shape = StockBlankShapeRound
				blank.Width = 0
				blank.Height = 0
				blank.Diameter = 20
			},
		},
		{
			name:    "negative piece price",
			change:  func(blank *StockBlank) { blank.PricePerPiece = -1 },
			wantErr: "price per piece",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blank := testRectangularBlank()
			test.change(&blank)
			assertMaterialError(t, blank.Validate(), test.wantErr)
		})
	}
}

func TestStockBlankAndMaterialMustBelongToSameCompany(t *testing.T) {
	material := testMaterial()
	blank := testRectangularBlank()
	blank.CompanyID = "another-company"

	assertMaterialError(t, blank.ValidateForMaterial(material), "same company")

	blank = testRectangularBlank()
	blank.MaterialID = "different-material"
	assertMaterialError(t, blank.ValidateForMaterial(material), "does not match material ID")
}

func assertMaterialError(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	if wantSubstring == "" {
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error containing %q", wantSubstring)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error = %q, want substring %q", err, wantSubstring)
	}
}

package model

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const ConsumableCutter = "cutter"

type Consumable struct {
	ID                string
	CompanyID         string
	Category          string
	Name              string
	Manufacturer      string
	ToolType          string
	DiameterMm        float64
	FluteCount        int
	ToolMaterial      string
	Coating           string
	LengthMm          float64
	StockQuantity     int
	UnitPrice         float64
	ResourceMinutes   float64
	UsedMinutesByUnit []float64 `json:",omitempty"`
	ResourceUnits     float64
	ResourceUnitName  string
	Currency          string
	SourceURL         string `json:",omitempty"`
	Attributes        json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (item Consumable) Validate() error {
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.CompanyID) == "" {
		return fmt.Errorf("consumable and company IDs are required")
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Category) == "" {
		return fmt.Errorf("consumable name and category are required")
	}
	if item.StockQuantity < 0 || !finiteNonNegative(item.UnitPrice) {
		return fmt.Errorf("consumable quantity and price must be non-negative")
	}
	if !finiteNonNegative(item.ResourceMinutes) || !finiteNonNegative(item.ResourceUnits) {
		return fmt.Errorf("consumable resource must be non-negative")
	}
	if item.UsedMinutesByUnit != nil && len(item.UsedMinutesByUnit) != item.StockQuantity {
		return fmt.Errorf("укажите наработку каждой фрезы на складе")
	}
	for _, used := range item.UsedMinutesByUnit {
		if !finiteNonNegative(used) {
			return fmt.Errorf("наработка фрезы должна быть конечным неотрицательным числом")
		}
	}
	if strings.TrimSpace(item.Currency) == "" {
		return fmt.Errorf("consumable currency is required")
	}
	if item.Category == ConsumableCutter {
		if !finiteNonNegative(item.DiameterMm) || item.DiameterMm <= 0 ||
			item.FluteCount <= 0 || !finiteNonNegative(item.LengthMm) || item.LengthMm <= 0 {
			return fmt.Errorf("cutter dimensions and flute count must be positive")
		}
	}
	if len(item.Attributes) == 0 {
		return fmt.Errorf("consumable attributes must be a JSON object")
	}
	var attributes map[string]any
	if err := json.Unmarshal(item.Attributes, &attributes); err != nil || attributes == nil {
		return fmt.Errorf("consumable attributes must be a JSON object")
	}
	return nil
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type StockBlankShape string

const (
	StockBlankShapeRectangular StockBlankShape = "rectangular"
	StockBlankShapeRound       StockBlankShape = "round"
)

type StockBlank struct {
	ID            string
	CompanyID     string
	MaterialID    string
	Name          string
	Shape         StockBlankShape
	Length        float64
	Width         float64
	Height        float64
	Diameter      float64
	PricePerPiece float64
	Currency      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (blank StockBlank) Validate() error {
	if strings.TrimSpace(blank.ID) == "" {
		return fmt.Errorf("stock blank ID is required")
	}
	if strings.TrimSpace(blank.CompanyID) == "" {
		return fmt.Errorf("stock blank company ID is required")
	}
	if strings.TrimSpace(blank.MaterialID) == "" {
		return fmt.Errorf("stock blank material ID is required")
	}
	if strings.TrimSpace(blank.Name) == "" {
		return fmt.Errorf("stock blank name is required")
	}
	if strings.TrimSpace(blank.Currency) == "" {
		return fmt.Errorf("stock blank currency is required")
	}
	if math.IsNaN(blank.PricePerPiece) || math.IsInf(blank.PricePerPiece, 0) || blank.PricePerPiece < 0 {
		return fmt.Errorf("stock blank price per piece must be non-negative")
	}

	switch blank.Shape {
	case StockBlankShapeRectangular:
		if err := validatePositiveDimension("length", blank.Length); err != nil {
			return err
		}
		if err := validatePositiveDimension("width", blank.Width); err != nil {
			return err
		}
		return validatePositiveDimension("height", blank.Height)
	case StockBlankShapeRound:
		if err := validatePositiveDimension("diameter", blank.Diameter); err != nil {
			return err
		}
		return validatePositiveDimension("length", blank.Length)
	default:
		return fmt.Errorf("unsupported stock blank shape %q", blank.Shape)
	}
}

func (blank StockBlank) ValidateForMaterial(material Material) error {
	if err := blank.Validate(); err != nil {
		return err
	}
	if err := material.Validate(); err != nil {
		return err
	}
	if blank.MaterialID != material.ID {
		return fmt.Errorf("stock blank material ID %q does not match material ID %q", blank.MaterialID, material.ID)
	}
	if blank.CompanyID != material.CompanyID {
		return fmt.Errorf("stock blank and material must belong to the same company")
	}
	return nil
}

func (blank StockBlank) VolumeMm3() (float64, error) {
	if err := blank.Validate(); err != nil {
		return 0, err
	}

	var volume float64
	switch blank.Shape {
	case StockBlankShapeRectangular:
		volume = blank.Length * blank.Width * blank.Height
	case StockBlankShapeRound:
		radius := blank.Diameter / 2
		volume = math.Pi * radius * radius * blank.Length
	}
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume <= 0 {
		return 0, fmt.Errorf("stock blank volume is invalid")
	}
	return volume, nil
}

func (blank StockBlank) MassKg(material Material) (float64, error) {
	if err := blank.ValidateForMaterial(material); err != nil {
		return 0, err
	}
	volume, err := blank.VolumeMm3()
	if err != nil {
		return 0, err
	}
	mass := volume * material.Density / 1_000_000_000
	if math.IsNaN(mass) || math.IsInf(mass, 0) || mass <= 0 {
		return 0, fmt.Errorf("stock blank mass is invalid")
	}
	return mass, nil
}

func (blank StockBlank) MaterialCost(material Material) (float64, error) {
	if err := blank.ValidateForMaterial(material); err != nil {
		return 0, err
	}
	if blank.PricePerPiece > 0 {
		return blank.PricePerPiece, nil
	}
	if material.PricePerKg <= 0 {
		return 0, fmt.Errorf("stock blank price per piece is not set and material price per kg is unavailable")
	}
	if blank.Currency != material.Currency {
		return 0, fmt.Errorf("stock blank and material currencies must match when calculating cost by mass")
	}
	mass, err := blank.MassKg(material)
	if err != nil {
		return 0, err
	}
	cost := mass * material.PricePerKg
	if math.IsNaN(cost) || math.IsInf(cost, 0) || cost <= 0 {
		return 0, fmt.Errorf("stock blank material cost is invalid")
	}
	return cost, nil
}

func validatePositiveDimension(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return fmt.Errorf("stock blank %s must be greater than zero", name)
	}
	return nil
}

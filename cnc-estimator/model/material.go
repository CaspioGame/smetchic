package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type MaterialCategory string

const (
	MaterialCategoryAluminum       MaterialCategory = "aluminum"
	MaterialCategorySteel          MaterialCategory = "steel"
	MaterialCategoryStainlessSteel MaterialCategory = "stainless_steel"
	MaterialCategoryBrass          MaterialCategory = "brass"
	MaterialCategoryCopper         MaterialCategory = "copper"
	MaterialCategoryPlastic        MaterialCategory = "plastic"
	MaterialCategoryOther          MaterialCategory = "other"
)

type Material struct {
	ID         string
	CompanyID  string
	Name       string
	Grade      string
	Category   MaterialCategory
	Density    float64
	PricePerKg float64
	Currency   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (material Material) Validate() error {
	if strings.TrimSpace(material.ID) == "" {
		return fmt.Errorf("material ID is required")
	}
	if strings.TrimSpace(material.CompanyID) == "" {
		return fmt.Errorf("material company ID is required")
	}
	if strings.TrimSpace(material.Name) == "" {
		return fmt.Errorf("material name is required")
	}
	if strings.TrimSpace(material.Grade) == "" {
		return fmt.Errorf("material grade is required")
	}
	switch material.Category {
	case MaterialCategoryAluminum, MaterialCategorySteel, MaterialCategoryStainlessSteel,
		MaterialCategoryBrass, MaterialCategoryCopper, MaterialCategoryPlastic, MaterialCategoryOther:
	default:
		return fmt.Errorf("unsupported material category %q", material.Category)
	}
	if math.IsNaN(material.Density) || math.IsInf(material.Density, 0) || material.Density <= 0 {
		return fmt.Errorf("material density must be greater than zero")
	}
	if math.IsNaN(material.PricePerKg) || math.IsInf(material.PricePerKg, 0) || material.PricePerKg < 0 {
		return fmt.Errorf("material price per kg must be non-negative")
	}
	if strings.TrimSpace(material.Currency) == "" {
		return fmt.Errorf("material currency is required")
	}
	return nil
}

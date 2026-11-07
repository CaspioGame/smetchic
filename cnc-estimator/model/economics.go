package model

import (
	"fmt"
	"math"
	"strings"
)

const (
	OverheadMonthly   = "monthly"
	OverheadAnnual    = "annual"
	OverheadUtilities = "utilities"
	OverheadOther     = "other"
)

const (
	AllocationByProductionHours = "by_production_hours"
)

type CompanyEconomics struct {
	CompanyID              string
	DefaultMarkupPercent   float64
	MonthlyRent            float64
	RentAllocationMethod   string
	MonthlyProductionHours float64
	Currency               string
}

func (item CompanyEconomics) Validate() error {
	if strings.TrimSpace(item.CompanyID) == "" {
		return fmt.Errorf("company ID is required")
	}
	if !finiteAtLeastZero(item.DefaultMarkupPercent) || !finiteAtLeastZero(item.MonthlyRent) {
		return fmt.Errorf("markup and monthly rent must be finite and non-negative")
	}
	if !finiteAtLeastZero(item.MonthlyProductionHours) ||
		(item.MonthlyRent > 0 && item.MonthlyProductionHours <= 0) {
		return fmt.Errorf("monthly production hours must be positive when allocating rent")
	}
	if item.RentAllocationMethod != AllocationByProductionHours {
		return fmt.Errorf("unsupported rent allocation method %q", item.RentAllocationMethod)
	}
	if !validCurrency(item.Currency) {
		return fmt.Errorf("currency must be a three-letter uppercase code")
	}
	return nil
}

type EquipmentEconomics struct {
	CompanyID              string
	EquipmentID            string
	PurchasePrice          float64
	ResidualValue          float64
	UsefulLifeYears        float64
	AnnualAvailableHours   float64
	ConsumptionKW          float64
	ElectricityPricePerKWh float64
	AnnualMaintenanceCost  float64
	AnnualRepairCost       float64
	AnnualOtherCost        float64
}

func (item EquipmentEconomics) Validate() error {
	if strings.TrimSpace(item.CompanyID) == "" || strings.TrimSpace(item.EquipmentID) == "" {
		return fmt.Errorf("company and equipment IDs are required")
	}
	for name, value := range map[string]float64{
		"purchase price": item.PurchasePrice, "residual value": item.ResidualValue,
		"useful life": item.UsefulLifeYears, "annual available hours": item.AnnualAvailableHours,
		"power consumption": item.ConsumptionKW, "electricity price": item.ElectricityPricePerKWh,
		"annual maintenance": item.AnnualMaintenanceCost, "annual repair": item.AnnualRepairCost,
		"annual other cost": item.AnnualOtherCost,
	} {
		if !finiteAtLeastZero(value) {
			return fmt.Errorf("%s must be finite and non-negative", name)
		}
	}
	if item.ResidualValue > item.PurchasePrice {
		return fmt.Errorf("residual value cannot exceed purchase price")
	}
	if item.UsefulLifeYears <= 0 || item.AnnualAvailableHours <= 0 {
		return fmt.Errorf("useful life and annual available hours must be positive")
	}
	return nil
}

type PersonnelRate struct {
	ID                    string
	CompanyID             string
	Role                  string
	MonthlyCost           float64
	MonthlyAvailableHours float64
	Currency              string
}

func (item PersonnelRate) Validate() error {
	if strings.TrimSpace(item.CompanyID) == "" || strings.TrimSpace(item.Role) == "" {
		return fmt.Errorf("company and personnel role are required")
	}
	switch item.Role {
	case "operator", "programmer", "setup":
	default:
		return fmt.Errorf("unsupported personnel role %q", item.Role)
	}
	if !finiteAtLeastZero(item.MonthlyCost) || !finitePositive(item.MonthlyAvailableHours) {
		return fmt.Errorf("monthly cost must be non-negative and available hours must be positive")
	}
	if !validCurrency(item.Currency) {
		return fmt.Errorf("currency must be a three-letter uppercase code")
	}
	return nil
}

type CompanyOverhead struct {
	ID               string
	CompanyID        string
	Name             string
	Category         string
	Amount           float64
	Period           string
	AllocationMethod string
	Active           bool
	Currency         string
}

func (item CompanyOverhead) Validate() error {
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.CompanyID) == "" ||
		strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("overhead ID, company ID, and name are required")
	}
	if item.Category != OverheadUtilities && item.Category != OverheadOther {
		return fmt.Errorf("unsupported overhead category %q", item.Category)
	}
	if !finiteAtLeastZero(item.Amount) {
		return fmt.Errorf("overhead amount must be finite and non-negative")
	}
	if item.Period != OverheadMonthly && item.Period != OverheadAnnual {
		return fmt.Errorf("unsupported overhead period %q", item.Period)
	}
	if item.AllocationMethod != AllocationByProductionHours {
		return fmt.Errorf("unsupported overhead allocation method %q", item.AllocationMethod)
	}
	if !validCurrency(item.Currency) {
		return fmt.Errorf("currency must be a three-letter uppercase code")
	}
	return nil
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func finiteAtLeastZero(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func finitePositive(value float64) bool {
	return finiteAtLeastZero(value) && value > 0
}

package costing

import (
	"fmt"
	"math"

	"smetchic/model"
)

type ToolUse struct {
	Name            string
	PurchasePrice   float64
	ResourceMinutes float64
	UsedMinutes     float64
}

type Input struct {
	Quantity                 int
	Currency                 string
	MachineTimeMinutes       float64
	LegacyMachineRatePerHour float64
	EquipmentEconomics       *model.EquipmentEconomics
	OperatorRate             *model.PersonnelRate
	ProgrammingRate          *model.PersonnelRate
	ProgrammingHours         float64
	SetupTimeMinutes         float64
	SetupCostPerHour         float64
	MaterialCostPerPiece     float64
	MaterialPriceSource      string
	Tools                    []ToolUse
	CompanyEconomics         model.CompanyEconomics
	Overheads                []model.CompanyOverhead
}

func Calculate(input Input) (model.ProductionCostBreakdown, error) {
	if input.Quantity <= 0 {
		return model.ProductionCostBreakdown{}, fmt.Errorf("quantity must be greater than zero")
	}
	if input.Currency == "" {
		return model.ProductionCostBreakdown{}, fmt.Errorf("currency is required")
	}
	if err := nonNegative("machine time", input.MachineTimeMinutes); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := nonNegative("legacy machine rate", input.LegacyMachineRatePerHour); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := nonNegative("programming hours", input.ProgrammingHours); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := nonNegative("setup time", input.SetupTimeMinutes); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := nonNegative("setup hourly cost", input.SetupCostPerHour); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := nonNegative("material cost", input.MaterialCostPerPiece); err != nil {
		return model.ProductionCostBreakdown{}, err
	}
	if err := input.CompanyEconomics.Validate(); err != nil {
		return model.ProductionCostBreakdown{}, fmt.Errorf("company economics: %w", err)
	}

	result := model.ProductionCostBreakdown{
		Quantity: input.Quantity, Currency: input.Currency,
		MaterialPriceSource: input.MaterialPriceSource,
	}
	units := float64(input.Quantity)
	machineHours := input.MachineTimeMinutes / 60
	if input.EquipmentEconomics == nil {
		result.MachineCost = input.LegacyMachineRatePerHour * machineHours * units
		result.Warnings = append(result.Warnings, "Equipment economics are not configured; legacy machine hourly rate is used.")
	} else {
		result.EquipmentEconomicsApplied = true
		result.Warnings = append(result.Warnings, "Legacy machine hourly rate is excluded because detailed equipment economics are configured.")
		equipment := *input.EquipmentEconomics
		if err := equipment.Validate(); err != nil {
			return model.ProductionCostBreakdown{}, fmt.Errorf("equipment economics: %w", err)
		}
		usefulLifeHours := equipment.UsefulLifeYears * equipment.AnnualAvailableHours
		result.DepreciationCost = (equipment.PurchasePrice - equipment.ResidualValue) /
			usefulLifeHours * machineHours * units
		result.ElectricityCost = equipment.ConsumptionKW * equipment.ElectricityPricePerKWh *
			machineHours * units
		result.MaintenanceCost = equipment.AnnualMaintenanceCost /
			equipment.AnnualAvailableHours * machineHours * units
		result.RepairCost = equipment.AnnualRepairCost /
			equipment.AnnualAvailableHours * machineHours * units
		result.OtherEquipmentCost = equipment.AnnualOtherCost /
			equipment.AnnualAvailableHours * machineHours * units
		result.MachineCost = result.DepreciationCost + result.ElectricityCost +
			result.MaintenanceCost + result.RepairCost + result.OtherEquipmentCost
	}

	result.MaterialCost = input.MaterialCostPerPiece * units
	result.OperatorCost = hourlyCost(input.OperatorRate) * input.MachineTimeMinutes / 60 * units
	if input.OperatorRate == nil {
		result.Warnings = append(result.Warnings, "Operator rate is not configured; operator cost is excluded.")
	}
	result.ProgrammingCost = hourlyCost(input.ProgrammingRate) * input.ProgrammingHours
	if input.ProgrammingHours > 0 && input.ProgrammingRate == nil {
		result.Warnings = append(result.Warnings, "Programming rate is not configured; programming cost is excluded.")
	}
	result.SetupCost = input.SetupCostPerHour * input.SetupTimeMinutes / 60
	if input.SetupTimeMinutes > 0 && input.SetupCostPerHour == 0 {
		result.Warnings = append(result.Warnings, "Setup cost rate is not configured; setup cost is excluded.")
	}

	for _, tool := range input.Tools {
		if err := nonNegative("tool purchase price", tool.PurchasePrice); err != nil {
			return model.ProductionCostBreakdown{}, err
		}
		if err := nonNegative("tool resource", tool.ResourceMinutes); err != nil {
			return model.ProductionCostBreakdown{}, err
		}
		if err := nonNegative("tool usage", tool.UsedMinutes); err != nil {
			return model.ProductionCostBreakdown{}, err
		}
		if tool.UsedMinutes == 0 {
			continue
		}
		if tool.ResourceMinutes == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Tool %q resource is not configured; tooling cost is excluded.", tool.Name))
			continue
		}
		result.ToolingCost += tool.PurchasePrice / tool.ResourceMinutes * tool.UsedMinutes * units
	}

	if input.CompanyEconomics.MonthlyRent > 0 {
		result.RentCost = input.CompanyEconomics.MonthlyRent /
			input.CompanyEconomics.MonthlyProductionHours * machineHours * units
	} else {
		result.Warnings = append(result.Warnings, "Rent is not configured; rent cost is excluded.")
	}
	for _, overhead := range input.Overheads {
		if !overhead.Active {
			continue
		}
		if err := overhead.Validate(); err != nil {
			return model.ProductionCostBreakdown{}, fmt.Errorf("overhead %q: %w", overhead.Name, err)
		}
		periodHours := input.CompanyEconomics.MonthlyProductionHours
		amount := overhead.Amount
		if overhead.Period == model.OverheadAnnual {
			periodHours *= 12
		}
		if periodHours <= 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Overhead %q has no production-hour allocation basis; excluded.", overhead.Name))
			continue
		}
		cost := amount / periodHours * machineHours * units
		if overhead.Category == model.OverheadUtilities {
			result.UtilitiesCost += cost
		} else {
			result.OtherOverheadCost += cost
		}
	}

	result.BatchCost = result.ProgrammingCost + result.SetupCost
	result.PerPieceCost = (result.MaterialCost + result.MachineCost + result.OperatorCost +
		result.ToolingCost + result.RentCost + result.UtilitiesCost + result.OtherOverheadCost) / units
	result.TotalCost = result.BatchCost + result.PerPieceCost*units
	result.Profit = result.TotalCost * input.CompanyEconomics.DefaultMarkupPercent / 100
	result.MarkupPercent = input.CompanyEconomics.DefaultMarkupPercent
	result.SellingPrice = result.TotalCost + result.Profit
	result.SellingPricePerPiece = result.SellingPrice / units
	for name, value := range map[string]float64{
		"total cost": result.TotalCost, "selling price": result.SellingPrice,
		"profit": result.Profit, "per-piece cost": result.PerPieceCost,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return model.ProductionCostBreakdown{}, fmt.Errorf("calculated %s is not finite", name)
		}
	}
	return result, nil
}

func hourlyCost(rate *model.PersonnelRate) float64 {
	if rate == nil || rate.MonthlyAvailableHours == 0 {
		return 0
	}
	return rate.MonthlyCost / rate.MonthlyAvailableHours
}

func nonNegative(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("%s must be a finite non-negative number", name)
	}
	return nil
}

package estimator

import (
	"fmt"

	"smetchic/calculator"
	"smetchic/model"
	"smetchic/planner"
)

func Calculate(part model.Part, equipment model.Equipment, settings model.EstimateSettings) (model.Estimate, error) {
	if settings.EquipmentID != equipment.ID {
		return model.Estimate{}, fmt.Errorf("выбранное оборудование %q не совпадает с оборудованием расчёта %q", settings.EquipmentID, equipment.ID)
	}

	operations, err := planner.BuildOperations(part, equipment)
	if err != nil {
		return model.Estimate{}, err
	}

	totalMachineTime := calculator.CalculateOperationTimes(operations)
	machineCost, totalCost, suggestedPrice := calculator.CalculateCosts(totalMachineTime, equipment, settings.MarkupPercent)
	return model.Estimate{
		Part:             part,
		Operations:       operations,
		Settings:         settings,
		TotalMachineTime: totalMachineTime,
		MachineCost:      machineCost,
		TotalCost:        totalCost,
		SuggestedPrice:   suggestedPrice,
		Equipment:        equipment,
	}, nil
}

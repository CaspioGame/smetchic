package estimator

import (
	"smetchic/calculator"
	"smetchic/model"
	"smetchic/planner"
)

func Calculate(part model.Part, machine model.Machine) (model.Estimate, error) {
	operations, err := planner.BuildOperations(part, machine)
	if err != nil {
		return model.Estimate{}, err
	}

	totalMachineTime := calculator.CalculateOperationTimes(operations)
	machineCost, totalCost, suggestedPrice := calculator.CalculateCosts(totalMachineTime, machine)
	return model.Estimate{
		Part:             part,
		Operations:       operations,
		TotalMachineTime: totalMachineTime,
		MachineCost:      machineCost,
		TotalCost:        totalCost,
		SuggestedPrice:   suggestedPrice,
		Machine:          machine,
	}, nil
}

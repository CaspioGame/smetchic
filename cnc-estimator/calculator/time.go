package calculator

import "smetchic/model"

func CalculateOperationTime(operation model.Operation) float64 {
	if operation.CuttingParameters.Feed <= 0 || operation.EstimatedToolpathLength <= 0 {
		return 0
	}
	return operation.EstimatedToolpathLength / operation.CuttingParameters.Feed
}

func CalculateOperationTimes(operations []model.Operation) float64 {
	var totalMachineTime float64
	for i := range operations {
		operation := &operations[i]
		maxFeed := operation.Equipment.EquipmentModel.MaxFeed
		if maxFeed > 0 && operation.CuttingParameters.Feed > maxFeed {
			operation.CuttingParameters.Feed = maxFeed
		}
		operation.TimeMinutes = CalculateOperationTime(*operation)
		totalMachineTime += operation.TimeMinutes
	}
	return totalMachineTime
}

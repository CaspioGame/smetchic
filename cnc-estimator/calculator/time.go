package calculator

import "smetchic/model"

func CalculateOperationTime(operation model.Operation) float64 {
	if operation.Feed <= 0 || operation.EstimatedToolpathLength <= 0 {
		return 0
	}
	return operation.EstimatedToolpathLength / operation.Feed
}

func CalculateOperationTimes(operations []model.Operation) float64 {
	var totalMachineTime float64
	for i := range operations {
		operation := &operations[i]
		if operation.Machine.MaxFeed > 0 && operation.Feed > operation.Machine.MaxFeed {
			operation.Feed = operation.Machine.MaxFeed
		}
		operation.TimeMinutes = CalculateOperationTime(*operation)
		totalMachineTime += operation.TimeMinutes
	}
	return totalMachineTime
}

package calculator

import (
	"math"

	"smetchic/model"
)

func CalculateCosts(totalMachineTime float64, machine model.Machine) (machineCost, totalCost, suggestedPrice float64) {
	machineCost = totalMachineTime / 60 * machine.HourlyRate
	totalCost = machineCost
	suggestedPrice = math.Round(totalCost*1.5*100) / 100
	return machineCost, totalCost, suggestedPrice
}

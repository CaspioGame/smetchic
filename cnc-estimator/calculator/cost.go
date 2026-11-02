package calculator

import (
	"math"

	"smetchic/model"
)

func CalculateCosts(totalMachineTime float64, equipment model.Equipment, markupPercent float64) (machineCost, totalCost, suggestedPrice float64) {
	machineCost = totalMachineTime / 60 * equipment.MachineRatePerHour
	totalCost = machineCost
	suggestedPrice = math.Round(totalCost*(1+markupPercent/100)*100) / 100
	return machineCost, totalCost, suggestedPrice
}

package main

import (
	"fmt"
)

type Machine struct {
	ID           string
	Name         string
	WorkX        float64
	WorkY        float64
	WorkZ        float64
	SpindlePower float64
	MaxRPM       float64
	MaxFeed      float64
	HourlyRate   float64
}

type Material struct {
	ID      string
	Name    string
	PriceKg float64
}

type Tool struct {
	ID       string
	Name     string
	Diameter float64
	Teeth    int
	MaxRPM   float64
}

type Part struct {
	Width    float64
	Length   float64
	Height   float64
	Material Material
}

type Operation struct {
	Name       string
	Tool       Tool
	RPM        float64
	Feed       float64
	StepDown   float64
	StepOver   float64
	CuttingLen float64
}

func CalculateCuttingTime(length float64, feed float64) float64 {
	return length / feed
}
func CalculateOperationTime(op Operation) float64 {
	return op.CuttingLen / op.Feed
}
func main() {
	operation := Operation{
		Name:       "Черновая обработка полости",
		RPM:        18000,
		Feed:       1200,
		StepDown:   2,
		StepOver:   5,
		CuttingLen: 5000,
	}

	minutes := CalculateOperationTime(operation)

	fmt.Printf("Время: %.2f мин\n", minutes)

}

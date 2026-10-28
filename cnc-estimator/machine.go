package main

type Machine struct {
	ID           string
	Name         string
	WorkX        float64
	WorkY        float64
	WorkZ        float64
	SpindlePower float64
	MaxRPM       float64
	MaxFeed      float64

	// Стоимость работы станка за час
	HourlyRate float64
}

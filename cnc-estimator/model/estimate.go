package model

type Estimate struct {
	Part             Part
	Operations       []Operation
	TotalMachineTime float64
	MachineCost      float64
	TotalCost        float64
	SuggestedPrice   float64
	Machine          Machine
}

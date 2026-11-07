package model

type Estimate struct {
	Part             Part
	Operations       []Operation
	Settings         EstimateSettings
	TotalMachineTime float64
	MachineCost      float64
	TotalCost        float64
	SuggestedPrice   float64
	Equipment        Equipment
	CostBreakdown    *ProductionCostBreakdown `json:"cost_breakdown,omitempty"`
}

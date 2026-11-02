package model

type Operation struct {
	ID                      string
	FeatureID               string
	Name                    string
	Strategy                string
	Equipment               Equipment
	Tool                    Tool
	CuttingParameters       CuttingParameters
	EstimatedToolpathLength float64
	TimeMinutes             float64
}

type CuttingParameters struct {
	Feed     float64
	RPM      float64
	Depth    float64
	StepDown float64
	StepOver float64
}

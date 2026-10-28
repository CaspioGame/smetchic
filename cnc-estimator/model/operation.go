package model

type Operation struct {
	ID                      string
	FeatureID               string
	Name                    string
	Strategy                string
	Machine                 Machine
	Tool                    Tool
	Feed                    float64
	RPM                     float64
	EstimatedToolpathLength float64
	TimeMinutes             float64
}

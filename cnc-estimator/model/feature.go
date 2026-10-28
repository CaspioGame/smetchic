package model

type FeatureType string

const (
	FeaturePocket  FeatureType = "pocket"
	FeatureHole    FeatureType = "hole"
	FeatureContour FeatureType = "contour"
	FeatureChamfer FeatureType = "chamfer"
)

type Feature struct {
	ID          string
	Type        FeatureType
	Width       float64
	Length      float64
	Depth       float64
	Diameter    float64
	PositionX   float64
	PositionY   float64
	Tolerance   float64
	ChamferSize float64
}

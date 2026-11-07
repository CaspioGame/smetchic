package model

import "time"

type GlobalMaterial struct {
	ID               string
	Name             string
	Grade            string
	Category         MaterialCategory
	Density          float64
	ReferencePriceKg float64
	Currency         string
	Description      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

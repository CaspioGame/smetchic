package model

import (
	"fmt"
	"math"
	"strings"
)

type CompanySettings struct {
	CompanyID            string
	Currency             string
	DefaultMarkupPercent float64
}

func (settings CompanySettings) Validate() error {
	if strings.TrimSpace(settings.CompanyID) == "" {
		return fmt.Errorf("company settings company ID is required")
	}
	if strings.TrimSpace(settings.Currency) == "" {
		return fmt.Errorf("company settings currency is required")
	}
	if math.IsNaN(settings.DefaultMarkupPercent) ||
		math.IsInf(settings.DefaultMarkupPercent, 0) ||
		settings.DefaultMarkupPercent < 0 {
		return fmt.Errorf("default markup percent must be a non-negative number")
	}
	return nil
}

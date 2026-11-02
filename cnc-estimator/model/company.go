package model

import (
	"fmt"
	"strings"
	"time"
)

type Company struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (company Company) Validate() error {
	if strings.TrimSpace(company.ID) == "" {
		return fmt.Errorf("company ID is required")
	}
	if strings.TrimSpace(company.Name) == "" {
		return fmt.Errorf("company name is required")
	}
	return nil
}

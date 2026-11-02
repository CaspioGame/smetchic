package model

import (
	"fmt"
	"strings"
	"time"
)

type CompanyRole string

const (
	CompanyRoleOwner    CompanyRole = "owner"
	CompanyRoleAdmin    CompanyRole = "admin"
	CompanyRoleManager  CompanyRole = "manager"
	CompanyRoleOperator CompanyRole = "operator"
)

type CompanyMembership struct {
	ID        string
	CompanyID string
	UserID    string
	Role      CompanyRole
	CreatedAt time.Time
}

func (membership CompanyMembership) Validate() error {
	if strings.TrimSpace(membership.ID) == "" {
		return fmt.Errorf("membership ID is required")
	}
	if strings.TrimSpace(membership.CompanyID) == "" {
		return fmt.Errorf("membership company ID is required")
	}
	if strings.TrimSpace(membership.UserID) == "" {
		return fmt.Errorf("membership user ID is required")
	}
	switch membership.Role {
	case CompanyRoleOwner, CompanyRoleAdmin, CompanyRoleManager, CompanyRoleOperator:
		return nil
	default:
		return fmt.Errorf("unsupported company role %q", membership.Role)
	}
}

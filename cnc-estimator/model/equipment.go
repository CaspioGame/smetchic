package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type EquipmentStatus string

const (
	EquipmentStatusActive      EquipmentStatus = "active"
	EquipmentStatusMaintenance EquipmentStatus = "maintenance"
	EquipmentStatusInactive    EquipmentStatus = "inactive"
)

type Equipment struct {
	ID                 string
	CompanyID          string
	EquipmentModelID   string
	Name               string
	InventoryNumber    string
	SerialNumber       string
	Status             EquipmentStatus
	MachineRatePerHour float64
	CreatedAt          time.Time
	UpdatedAt          time.Time

	EquipmentModel EquipmentModel
}

func (equipment Equipment) Validate() error {
	if strings.TrimSpace(equipment.ID) == "" {
		return fmt.Errorf("equipment ID is required")
	}
	if strings.TrimSpace(equipment.CompanyID) == "" {
		return fmt.Errorf("equipment company ID is required")
	}
	if strings.TrimSpace(equipment.EquipmentModelID) == "" {
		return fmt.Errorf("equipment model ID is required")
	}
	if strings.TrimSpace(equipment.Name) == "" {
		return fmt.Errorf("equipment name is required")
	}
	switch equipment.Status {
	case EquipmentStatusActive, EquipmentStatusMaintenance, EquipmentStatusInactive:
	default:
		return fmt.Errorf("unsupported equipment status %q", equipment.Status)
	}
	if math.IsNaN(equipment.MachineRatePerHour) ||
		math.IsInf(equipment.MachineRatePerHour, 0) ||
		equipment.MachineRatePerHour <= 0 {
		return fmt.Errorf("equipment machine rate per hour must be greater than zero")
	}
	return nil
}

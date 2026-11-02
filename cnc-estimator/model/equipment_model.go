package model

import (
	"fmt"
	"math"
	"strings"
)

type EquipmentType string

const (
	EquipmentMilling   EquipmentType = "milling"
	EquipmentTurning   EquipmentType = "turning"
	EquipmentLaser     EquipmentType = "laser"
	Equipment3DPrinter EquipmentType = "3d_printer"
	EquipmentWelding   EquipmentType = "welding"
	EquipmentPress     EquipmentType = "press"
	EquipmentOther     EquipmentType = "other"
)

type EquipmentModel struct {
	ID           string
	Type         EquipmentType
	Manufacturer string
	Model        string
	WorkAreaX    float64
	WorkAreaY    float64
	WorkAreaZ    float64
	MaxRPM       float64
	SpindlePower float64
	MaxFeed      float64
}

func (equipmentModel EquipmentModel) Validate() error {
	if strings.TrimSpace(equipmentModel.ID) == "" {
		return fmt.Errorf("equipment model ID is required")
	}
	if strings.TrimSpace(equipmentModel.Manufacturer) == "" {
		return fmt.Errorf("equipment model manufacturer is required")
	}
	if strings.TrimSpace(equipmentModel.Model) == "" {
		return fmt.Errorf("equipment model name is required")
	}
	switch equipmentModel.Type {
	case EquipmentMilling, EquipmentTurning, EquipmentLaser, Equipment3DPrinter,
		EquipmentWelding, EquipmentPress, EquipmentOther:
	default:
		return fmt.Errorf("unsupported equipment type %q", equipmentModel.Type)
	}
	for name, value := range map[string]float64{
		"work area X":   equipmentModel.WorkAreaX,
		"work area Y":   equipmentModel.WorkAreaY,
		"work area Z":   equipmentModel.WorkAreaZ,
		"maximum RPM":   equipmentModel.MaxRPM,
		"spindle power": equipmentModel.SpindlePower,
		"maximum feed":  equipmentModel.MaxFeed,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("equipment model %s must be non-negative", name)
		}
	}
	return nil
}

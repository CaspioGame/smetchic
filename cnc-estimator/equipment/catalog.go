package equipment

import (
	"fmt"

	"smetchic/model"
)

type Catalog struct {
	models    map[string]model.EquipmentModel
	equipment map[string]model.Equipment
	order     []string
}

func NewCatalog(models []model.EquipmentModel, items []model.Equipment) (*Catalog, error) {
	catalog := &Catalog{
		models:    make(map[string]model.EquipmentModel, len(models)),
		equipment: make(map[string]model.Equipment, len(items)),
		order:     make([]string, 0, len(items)),
	}
	for _, equipmentModel := range models {
		if err := equipmentModel.Validate(); err != nil {
			return nil, err
		}
		if _, exists := catalog.models[equipmentModel.ID]; exists {
			return nil, fmt.Errorf("duplicate equipment model ID %q", equipmentModel.ID)
		}
		catalog.models[equipmentModel.ID] = equipmentModel
	}
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return nil, err
		}
		if _, exists := catalog.equipment[item.ID]; exists {
			return nil, fmt.Errorf("duplicate equipment ID %q", item.ID)
		}
		equipmentModel, exists := catalog.models[item.EquipmentModelID]
		if !exists {
			return nil, fmt.Errorf("equipment %q references unknown model %q", item.ID, item.EquipmentModelID)
		}
		item.EquipmentModel = equipmentModel
		catalog.equipment[item.ID] = item
		catalog.order = append(catalog.order, item.ID)
	}
	return catalog, nil
}

func DefaultCatalog() (*Catalog, error) {
	models := []model.EquipmentModel{
		{
			ID:           "dekart-s4040",
			Type:         model.EquipmentMilling,
			Manufacturer: "DeKart",
			Model:        "S4040",
			MaxFeed:      5000,
		},
	}
	items := []model.Equipment{
		{
			ID:                 "dekart-s4040",
			CompanyID:          "demo-company",
			EquipmentModelID:   "dekart-s4040",
			Name:               "DeKart S4040",
			Status:             model.EquipmentStatusActive,
			MachineRatePerHour: 1500,
		},
	}
	return NewCatalog(models, items)
}

func (catalog *Catalog) List() []model.Equipment {
	items := make([]model.Equipment, 0, len(catalog.order))
	for _, id := range catalog.order {
		items = append(items, catalog.equipment[id])
	}
	return items
}

func (catalog *Catalog) GetByID(id string) (model.Equipment, error) {
	item, exists := catalog.equipment[id]
	if !exists {
		return model.Equipment{}, fmt.Errorf("оборудование с ID %q не найдено", id)
	}
	return item, nil
}

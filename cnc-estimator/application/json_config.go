package application

import (
	"context"
	"fmt"

	"smetchic/model"
	"smetchic/storage"
)

type JSONConfigProvider struct {
	store *storage.Store
}

func NewJSONConfigProvider(store *storage.Store) *JSONConfigProvider {
	return &JSONConfigProvider{store: store}
}

func (provider *JSONConfigProvider) GetCompanySettings(_ context.Context, companyID string) (model.CompanySettings, error) {
	snapshot, err := provider.store.Snapshot(companyID)
	if err != nil {
		return model.CompanySettings{}, fmt.Errorf("load company settings: %w", err)
	}
	return snapshot.Settings, nil
}

func (provider *JSONConfigProvider) GetEquipment(_ context.Context, companyID, equipmentID string) (model.Equipment, error) {
	equipment, err := provider.store.GetEquipment(companyID, equipmentID)
	if err != nil {
		return model.Equipment{}, fmt.Errorf("load equipment: %w", err)
	}
	return equipment, nil
}

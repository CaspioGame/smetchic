package equipment

import (
	"strings"
	"testing"

	"smetchic/model"
)

func TestDefaultCatalogContainsDeKartS4040(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog() error = %v", err)
	}
	item, err := catalog.GetByID("dekart-s4040")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.Name != "DeKart S4040" || item.MachineRatePerHour != 1500 {
		t.Errorf("equipment = %+v, want DeKart S4040 at 1500 per hour", item)
	}
	if item.EquipmentModel.Type != model.EquipmentMilling ||
		item.EquipmentModel.Manufacturer != "DeKart" ||
		item.EquipmentModel.Model != "S4040" ||
		item.EquipmentModel.MaxFeed != 5000 {
		t.Errorf("equipment model = %+v, want DeKart S4040 technical data", item.EquipmentModel)
	}
}

func TestCatalogResolvesEquipmentModel(t *testing.T) {
	catalog, err := NewCatalog(
		[]model.EquipmentModel{{
			ID: "mill-model", Type: model.EquipmentMilling,
			Manufacturer: "Test", Model: "Mill", MaxFeed: 2500,
		}},
		[]model.Equipment{{
			ID: "mill-1", CompanyID: "company-1", EquipmentModelID: "mill-model",
			Name: "Production Mill", Status: model.EquipmentStatusActive, MachineRatePerHour: 2000,
		}},
	)
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}

	item, err := catalog.GetByID("mill-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.EquipmentModel.ID != item.EquipmentModelID || item.EquipmentModel.MaxFeed != 2500 {
		t.Errorf("equipment model relation = %+v", item.EquipmentModel)
	}
}

func TestGetByIDReturnsClearErrorForUnknownEquipment(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog() error = %v", err)
	}
	_, err = catalog.GetByID("unknown-equipment")
	if err == nil {
		t.Fatal("GetByID() expected an error for an unknown ID")
	}
	if !strings.Contains(err.Error(), "unknown-equipment") || !strings.Contains(err.Error(), "не найдено") {
		t.Errorf("GetByID() error = %q, want ID and not-found message", err)
	}
}

func TestNewCatalogRejectsEquipmentWithUnknownModel(t *testing.T) {
	_, err := NewCatalog(nil, []model.Equipment{{
		ID: "mill-1", CompanyID: "company-1", EquipmentModelID: "missing-model",
		Name: "Mill", Status: model.EquipmentStatusActive, MachineRatePerHour: 1500,
	}})
	if err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("NewCatalog() error = %v, want missing model reference error", err)
	}
}

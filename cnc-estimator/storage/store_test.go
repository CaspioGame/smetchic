package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"smetchic/model"
)

func TestNewStoreSeedsDemoRecordsOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	snapshot, err := store.Snapshot(DemoCompanyID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Company.Name != "Демонстрационная компания" ||
		snapshot.Settings.Currency != "RUB" ||
		snapshot.Settings.DefaultMarkupPercent != 50 {
		t.Fatalf("unexpected demo company defaults: %+v %+v", snapshot.Company, snapshot.Settings)
	}
	if len(snapshot.EquipmentModels) != 1 || snapshot.EquipmentModels[0].ID != "dekart-s4040" {
		t.Fatalf("unexpected equipment models: %+v", snapshot.EquipmentModels)
	}
	if len(snapshot.Equipment) != 1 ||
		snapshot.Equipment[0].ID != "dekart-s4040" ||
		snapshot.Equipment[0].MachineRatePerHour != 1500 {
		t.Fatalf("unexpected equipment seed: %+v", snapshot.Equipment)
	}

	snapshot.Company.Name = "Сохранённое название"
	snapshot.Settings.DefaultMarkupPercent = 75
	if err := store.SaveCompany(snapshot.Company, snapshot.Settings); err != nil {
		t.Fatalf("SaveCompany() error = %v", err)
	}
	if err := store.SaveMaterial(DemoCompanyID, testStoreMaterial()); err != nil {
		t.Fatalf("SaveMaterial() before restart error = %v", err)
	}
	demoEquipment := snapshot.Equipment[0]
	demoEquipment.MachineRatePerHour = 1800
	if err := store.SaveEquipment(DemoCompanyID, demoEquipment); err != nil {
		t.Fatalf("SaveEquipment() before restart error = %v", err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	reloaded, err := reopened.Snapshot(DemoCompanyID)
	if err != nil {
		t.Fatalf("reloaded Snapshot() error = %v", err)
	}
	if reloaded.Company.Name != "Сохранённое название" || reloaded.Settings.DefaultMarkupPercent != 75 {
		t.Fatalf("saved values were overwritten: %+v %+v", reloaded.Company, reloaded.Settings)
	}
	if len(reloaded.Equipment) != 1 || len(reloaded.EquipmentModels) != 1 {
		t.Fatalf("reopen created duplicate seeds: %+v", reloaded)
	}
	if len(reloaded.Materials) != 1 || reloaded.Materials[0].ID != "aluminum-6061" {
		t.Fatalf("material data did not persist across restart: %+v", reloaded.Materials)
	}
	if reloaded.Equipment[0].MachineRatePerHour != 1800 {
		t.Fatalf("saved equipment rate was overwritten by demo defaults: %+v", reloaded.Equipment[0])
	}
	for _, name := range []string{
		"companies.json", "company_settings.json", "equipment_models.json",
		"equipment.json", "materials.json", "stock_blanks.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected data file %s: %v", name, err)
		}
	}
}

func TestStoreEquipmentCRUD(t *testing.T) {
	store := testStore(t)
	equipment := model.Equipment{
		ID: "cnc-2", EquipmentModelID: "dekart-s4040", Name: "Фрезер №2",
		Status: model.EquipmentStatusMaintenance, MachineRatePerHour: 2200,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := store.SaveEquipment(DemoCompanyID, equipment); err != nil {
		t.Fatalf("SaveEquipment() error = %v", err)
	}
	got, err := store.GetEquipment(DemoCompanyID, equipment.ID)
	if err != nil {
		t.Fatalf("GetEquipment() error = %v", err)
	}
	if got.MachineRatePerHour != 2200 || got.EquipmentModel.Model != "S4040" {
		t.Fatalf("unexpected saved equipment: %+v", got)
	}
	if err := store.DeleteEquipment(DemoCompanyID, equipment.ID); err != nil {
		t.Fatalf("DeleteEquipment() error = %v", err)
	}
	if _, err := store.GetEquipment(DemoCompanyID, equipment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetEquipment() after delete error = %v, want ErrNotFound", err)
	}
}

func TestStoreMaterialCRUDAndReferencedDeleteGuard(t *testing.T) {
	store := testStore(t)
	material := testStoreMaterial()
	if err := store.SaveMaterial(DemoCompanyID, material); err != nil {
		t.Fatalf("SaveMaterial() error = %v", err)
	}
	if err := store.DeleteMaterial(DemoCompanyID, material.ID); err != nil {
		t.Fatalf("DeleteMaterial() before references error = %v", err)
	}
	if err := store.SaveMaterial(DemoCompanyID, material); err != nil {
		t.Fatalf("SaveMaterial() second time error = %v", err)
	}
	if err := store.SaveStockBlank(DemoCompanyID, testStoreBlank()); err != nil {
		t.Fatalf("SaveStockBlank() error = %v", err)
	}
	if err := store.DeleteMaterial(DemoCompanyID, material.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("DeleteMaterial() with stock blank error = %v, want ErrReferenced", err)
	}
	if err := store.DeleteStockBlank(DemoCompanyID, "blank-1"); err != nil {
		t.Fatalf("DeleteStockBlank() error = %v", err)
	}
	if err := store.DeleteMaterial(DemoCompanyID, material.ID); err != nil {
		t.Fatalf("DeleteMaterial() after blank delete error = %v", err)
	}
}

func TestStoreRejectsCrossCompanyMaterialReference(t *testing.T) {
	dir := t.TempDir()
	companies := []model.Company{
		{ID: DemoCompanyID, Name: "Demo"},
		{ID: "another-company", Name: "Another"},
	}
	encodedCompanies, err := json.Marshal(companies)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "companies.json"), encodedCompanies, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	material := testStoreMaterial()
	material.CompanyID = "another-company"
	if err := store.SaveMaterial("another-company", material); err != nil {
		t.Fatalf("SaveMaterial() for second company error = %v", err)
	}
	if err := store.SaveStockBlank(DemoCompanyID, testStoreBlank()); err == nil {
		t.Fatal("SaveStockBlank() accepted a material from another company")
	}
}

func TestStoreConcurrentMaterialWritesRemainValidJSON(t *testing.T) {
	store := testStore(t)
	const writes = 16
	var wait sync.WaitGroup
	errorsFound := make(chan error, writes)
	for i := 0; i < writes; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			material := testStoreMaterial()
			material.ID = "material-" + string(rune('a'+index))
			errorsFound <- store.SaveMaterial(DemoCompanyID, material)
		}(i)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Errorf("concurrent SaveMaterial() error = %v", err)
		}
	}
	if _, err := NewStore(store.dir); err != nil {
		t.Fatalf("reopening concurrently-written store: %v", err)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func testStoreMaterial() model.Material {
	return model.Material{
		ID: "aluminum-6061", CompanyID: DemoCompanyID, Name: "Алюминий",
		Grade: "6061", Category: model.MaterialCategoryAluminum,
		Density: 2700, PricePerKg: 500, Currency: "RUB",
	}
}

func testStoreBlank() model.StockBlank {
	return model.StockBlank{
		ID: "blank-1", CompanyID: DemoCompanyID, MaterialID: "aluminum-6061",
		Name: "Плита", Shape: model.StockBlankShapeRectangular,
		Length: 100, Width: 50, Height: 20, Currency: "RUB",
	}
}

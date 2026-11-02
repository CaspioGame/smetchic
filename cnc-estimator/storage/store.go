package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"smetchic/model"
)

const DemoCompanyID = "demo-company"

var (
	ErrNotFound        = errors.New("record not found")
	ErrReferenced      = errors.New("record is referenced")
	ErrCompanyMismatch = errors.New("record belongs to another company")
	ErrDuplicateID     = errors.New("record ID already exists")
)

type Snapshot struct {
	Company         model.Company
	Settings        model.CompanySettings
	EquipmentModels []model.EquipmentModel
	Equipment       []model.Equipment
	Materials       []model.Material
	StockBlanks     []model.StockBlank
}

type Store struct {
	mu              sync.RWMutex
	dir             string
	companies       []model.Company
	settings        []model.CompanySettings
	equipmentModels []model.EquipmentModel
	equipment       []model.Equipment
	materials       []model.Material
	stockBlanks     []model.StockBlank
}

func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("storage directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	store := &Store{dir: dir}
	files := []struct {
		name string
		load func(string) (bool, error)
	}{
		{"companies.json", func(path string) (bool, error) {
			return loadJSON(path, &store.companies)
		}},
		{"company_settings.json", func(path string) (bool, error) {
			return loadJSON(path, &store.settings)
		}},
		{"equipment_models.json", func(path string) (bool, error) {
			return loadJSON(path, &store.equipmentModels)
		}},
		{"equipment.json", func(path string) (bool, error) {
			return loadJSON(path, &store.equipment)
		}},
		{"materials.json", func(path string) (bool, error) {
			return loadJSON(path, &store.materials)
		}},
		{"stock_blanks.json", func(path string) (bool, error) {
			return loadJSON(path, &store.stockBlanks)
		}},
	}
	missing := make(map[string]bool)
	for _, file := range files {
		exists, err := file.load(filepath.Join(dir, file.name))
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", file.name, err)
		}
		missing[file.name] = !exists
	}
	if err := store.initialize(missing); err != nil {
		return nil, err
	}
	if err := store.validateData(); err != nil {
		return nil, fmt.Errorf("validate JSON data: %w", err)
	}
	return store, nil
}

func (store *Store) initialize(missing map[string]bool) error {
	now := time.Now().UTC()
	originalLengths := map[string]int{
		"companies.json":        len(store.companies),
		"company_settings.json": len(store.settings),
		"equipment_models.json": len(store.equipmentModels),
		"equipment.json":        len(store.equipment),
		"materials.json":        len(store.materials),
		"stock_blanks.json":     len(store.stockBlanks),
	}
	if store.companies == nil {
		store.companies = []model.Company{}
	}
	if store.settings == nil {
		store.settings = []model.CompanySettings{}
	}
	if store.equipmentModels == nil {
		store.equipmentModels = []model.EquipmentModel{}
	}
	if store.equipment == nil {
		store.equipment = []model.Equipment{}
	}
	if store.materials == nil {
		store.materials = []model.Material{}
	}
	if store.stockBlanks == nil {
		store.stockBlanks = []model.StockBlank{}
	}
	if len(store.companies) == 0 {
		store.companies = append(store.companies, model.Company{
			ID: DemoCompanyID, Name: "Демонстрационная компания", CreatedAt: now, UpdatedAt: now,
		})
	}
	if err := store.ensureCompanySettings(); err != nil {
		return err
	}
	if !containsEquipmentModel(store.equipmentModels, "dekart-s4040") {
		store.equipmentModels = append(store.equipmentModels, model.EquipmentModel{
			ID: "dekart-s4040", Type: model.EquipmentMilling,
			Manufacturer: "DeKart", Model: "S4040", MaxFeed: 5000,
		})
	}
	if companyExists(store.companies, DemoCompanyID) && !hasEquipmentForCompany(store.equipment, DemoCompanyID) {
		equipmentID := "dekart-s4040"
		if containsEquipment(store.equipment, equipmentID) {
			equipmentID = "demo-dekart-s4040"
			for suffix := 2; containsEquipment(store.equipment, equipmentID); suffix++ {
				equipmentID = fmt.Sprintf("demo-dekart-s4040-%d", suffix)
			}
		}
		store.equipment = append(store.equipment, model.Equipment{
			ID: equipmentID, CompanyID: DemoCompanyID, EquipmentModelID: "dekart-s4040",
			Name: "DeKart S4040", Status: model.EquipmentStatusActive,
			MachineRatePerHour: 1500, CreatedAt: now, UpdatedAt: now,
		})
	}

	writers := []struct {
		name string
		data any
	}{
		{"companies.json", store.companies},
		{"company_settings.json", store.settings},
		{"equipment_models.json", store.equipmentModels},
		{"equipment.json", equipmentRecords(store.equipment)},
		{"materials.json", store.materials},
		{"stock_blanks.json", store.stockBlanks},
	}
	for _, file := range writers {
		if missing[file.name] || originalLengths[file.name] != collectionLength(file.name, store) {
			if err := store.write(file.name, file.data); err != nil {
				return fmt.Errorf("initialize %s: %w", file.name, err)
			}
		}
	}
	return nil
}

func collectionLength(name string, store *Store) int {
	switch name {
	case "companies.json":
		return len(store.companies)
	case "company_settings.json":
		return len(store.settings)
	case "equipment_models.json":
		return len(store.equipmentModels)
	case "equipment.json":
		return len(store.equipment)
	case "materials.json":
		return len(store.materials)
	case "stock_blanks.json":
		return len(store.stockBlanks)
	default:
		return 0
	}
}

func (store *Store) ensureCompanySettings() error {
	for _, company := range store.companies {
		if settingsForCompany(store.settings, company.ID) == nil {
			store.settings = append(store.settings, model.CompanySettings{
				CompanyID: company.ID, Currency: "RUB", DefaultMarkupPercent: 50,
			})
		}
	}
	for _, settings := range store.settings {
		if !companyExists(store.companies, settings.CompanyID) {
			return fmt.Errorf("company settings reference unknown company %q", settings.CompanyID)
		}
		if err := settings.Validate(); err != nil {
			return fmt.Errorf("invalid settings for company %q: %w", settings.CompanyID, err)
		}
	}
	return nil
}

func (store *Store) validateData() error {
	companyIDs := make(map[string]bool, len(store.companies))
	for _, company := range store.companies {
		if err := company.Validate(); err != nil {
			return err
		}
		if companyIDs[company.ID] {
			return fmt.Errorf("duplicate company ID %q", company.ID)
		}
		companyIDs[company.ID] = true
	}

	models := make(map[string]bool, len(store.equipmentModels))
	for _, equipmentModel := range store.equipmentModels {
		if err := equipmentModel.Validate(); err != nil {
			return err
		}
		if models[equipmentModel.ID] {
			return fmt.Errorf("duplicate equipment model ID %q", equipmentModel.ID)
		}
		models[equipmentModel.ID] = true
	}

	settingsIDs := make(map[string]bool, len(store.settings))
	for _, settings := range store.settings {
		if err := settings.Validate(); err != nil {
			return err
		}
		if !companyIDs[settings.CompanyID] {
			return fmt.Errorf("settings reference unknown company %q", settings.CompanyID)
		}
		if settingsIDs[settings.CompanyID] {
			return fmt.Errorf("duplicate settings for company %q", settings.CompanyID)
		}
		settingsIDs[settings.CompanyID] = true
	}

	equipmentIDs := make(map[string]bool, len(store.equipment))
	for _, item := range store.equipment {
		if err := item.Validate(); err != nil {
			return err
		}
		if !companyIDs[item.CompanyID] || !models[item.EquipmentModelID] {
			return fmt.Errorf("equipment %q references an unknown company or model", item.ID)
		}
		if equipmentIDs[item.ID] {
			return fmt.Errorf("duplicate equipment ID %q", item.ID)
		}
		equipmentIDs[item.ID] = true
	}

	materials := make(map[string]model.Material, len(store.materials))
	for _, item := range store.materials {
		if err := item.Validate(); err != nil {
			return err
		}
		if !companyIDs[item.CompanyID] {
			return fmt.Errorf("material %q references unknown company %q", item.ID, item.CompanyID)
		}
		if _, exists := materials[item.ID]; exists {
			return fmt.Errorf("duplicate material ID %q", item.ID)
		}
		materials[item.ID] = item
	}

	blankIDs := make(map[string]bool, len(store.stockBlanks))
	for _, blank := range store.stockBlanks {
		material, exists := materials[blank.MaterialID]
		if !exists {
			return fmt.Errorf("stock blank %q references unknown material %q", blank.ID, blank.MaterialID)
		}
		if err := blank.ValidateForMaterial(material); err != nil {
			return err
		}
		if blankIDs[blank.ID] {
			return fmt.Errorf("duplicate stock blank ID %q", blank.ID)
		}
		blankIDs[blank.ID] = true
	}
	return nil
}

func (store *Store) Snapshot(companyID string) (Snapshot, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	company, ok := findCompany(store.companies, companyID)
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	settings := settingsForCompany(store.settings, companyID)
	if settings == nil {
		return Snapshot{}, fmt.Errorf("company settings not found")
	}
	models := make(map[string]model.EquipmentModel, len(store.equipmentModels))
	for _, equipmentModel := range store.equipmentModels {
		models[equipmentModel.ID] = equipmentModel
	}
	equipmentItems := make([]model.Equipment, 0)
	for _, item := range store.equipment {
		if item.CompanyID != companyID {
			continue
		}
		item.EquipmentModel = models[item.EquipmentModelID]
		equipmentItems = append(equipmentItems, item)
	}
	return Snapshot{
		Company: company, Settings: *settings, EquipmentModels: append([]model.EquipmentModel(nil), store.equipmentModels...),
		Equipment: equipmentItems, Materials: companyMaterials(store.materials, companyID),
		StockBlanks: companyStockBlanks(store.stockBlanks, companyID),
	}, nil
}

func (store *Store) FirstCompanyID() (string, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.companies) == 0 {
		return "", ErrNotFound
	}
	return store.companies[0].ID, nil
}

func (store *Store) GetEquipment(companyID, id string) (model.Equipment, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, item := range store.equipment {
		if item.ID != id {
			continue
		}
		if item.CompanyID != companyID {
			return model.Equipment{}, ErrCompanyMismatch
		}
		for _, equipmentModel := range store.equipmentModels {
			if equipmentModel.ID == item.EquipmentModelID {
				item.EquipmentModel = equipmentModel
				return item, nil
			}
		}
		return model.Equipment{}, fmt.Errorf("equipment model %q not found", item.EquipmentModelID)
	}
	return model.Equipment{}, ErrNotFound
}

func (store *Store) SaveEquipment(companyID string, item model.Equipment) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return err
	}
	if !companyExists(store.companies, companyID) {
		return ErrNotFound
	}
	if !containsEquipmentModel(store.equipmentModels, item.EquipmentModelID) {
		return fmt.Errorf("equipment model not found")
	}
	index := -1
	for i := range store.equipment {
		if store.equipment[i].ID == item.ID {
			if store.equipment[i].CompanyID != companyID {
				return ErrCompanyMismatch
			}
			index = i
			item.CreatedAt = store.equipment[i].CreatedAt
			break
		}
	}
	if index < 0 && containsEquipment(store.equipment, item.ID) {
		return ErrDuplicateID
	}
	candidate := append([]model.Equipment(nil), store.equipment...)
	item.EquipmentModel = model.EquipmentModel{}
	if index < 0 {
		candidate = append(candidate, item)
	} else {
		candidate[index] = item
	}
	if err := store.write("equipment.json", equipmentRecords(candidate)); err != nil {
		return err
	}
	store.equipment = candidate
	return nil
}

func (store *Store) DeleteEquipment(companyID, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	index := findEquipmentIndex(store.equipment, companyID, id)
	if index == -2 {
		return ErrCompanyMismatch
	}
	if index < 0 {
		return ErrNotFound
	}
	candidate := removeEquipment(store.equipment, index)
	if err := store.write("equipment.json", equipmentRecords(candidate)); err != nil {
		return err
	}
	store.equipment = candidate
	return nil
}

func (store *Store) SaveMaterial(companyID string, material model.Material) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	material.CompanyID = companyID
	if err := material.Validate(); err != nil {
		return err
	}
	if !companyExists(store.companies, companyID) {
		return ErrNotFound
	}
	index := -1
	for i := range store.materials {
		if store.materials[i].ID == material.ID {
			if store.materials[i].CompanyID != companyID {
				return ErrCompanyMismatch
			}
			index = i
			material.CreatedAt = store.materials[i].CreatedAt
			break
		}
	}
	if index < 0 && containsMaterial(store.materials, material.ID) {
		return ErrDuplicateID
	}
	candidate := append([]model.Material(nil), store.materials...)
	if index < 0 {
		candidate = append(candidate, material)
	} else {
		candidate[index] = material
	}
	if err := store.write("materials.json", candidate); err != nil {
		return err
	}
	store.materials = candidate
	return nil
}

func (store *Store) DeleteMaterial(companyID, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	index := findMaterialIndex(store.materials, companyID, id)
	if index == -2 {
		return ErrCompanyMismatch
	}
	if index < 0 {
		return ErrNotFound
	}
	for _, blank := range store.stockBlanks {
		if blank.CompanyID == companyID && blank.MaterialID == id {
			return ErrReferenced
		}
	}
	candidate := removeMaterial(store.materials, index)
	if err := store.write("materials.json", candidate); err != nil {
		return err
	}
	store.materials = candidate
	return nil
}

func (store *Store) SaveStockBlank(companyID string, blank model.StockBlank) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	blank.CompanyID = companyID
	var material *model.Material
	for i := range store.materials {
		if store.materials[i].ID == blank.MaterialID {
			if store.materials[i].CompanyID != companyID {
				return ErrCompanyMismatch
			}
			material = &store.materials[i]
			break
		}
	}
	if material == nil {
		return fmt.Errorf("material not found")
	}
	if err := blank.ValidateForMaterial(*material); err != nil {
		return err
	}
	index := -1
	for i := range store.stockBlanks {
		if store.stockBlanks[i].ID == blank.ID {
			if store.stockBlanks[i].CompanyID != companyID {
				return ErrCompanyMismatch
			}
			index = i
			blank.CreatedAt = store.stockBlanks[i].CreatedAt
			break
		}
	}
	if index < 0 && containsStockBlank(store.stockBlanks, blank.ID) {
		return ErrDuplicateID
	}
	candidate := append([]model.StockBlank(nil), store.stockBlanks...)
	if index < 0 {
		candidate = append(candidate, blank)
	} else {
		candidate[index] = blank
	}
	if err := store.write("stock_blanks.json", candidate); err != nil {
		return err
	}
	store.stockBlanks = candidate
	return nil
}

func (store *Store) DeleteStockBlank(companyID, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	index := findStockBlankIndex(store.stockBlanks, companyID, id)
	if index == -2 {
		return ErrCompanyMismatch
	}
	if index < 0 {
		return ErrNotFound
	}
	candidate := removeStockBlank(store.stockBlanks, index)
	if err := store.write("stock_blanks.json", candidate); err != nil {
		return err
	}
	store.stockBlanks = candidate
	return nil
}

func (store *Store) SaveCompany(company model.Company, settings model.CompanySettings) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	index := -1
	for i := range store.companies {
		if store.companies[i].ID == company.ID {
			index = i
			company.CreatedAt = store.companies[i].CreatedAt
			break
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	settings.CompanyID = company.ID
	if err := company.Validate(); err != nil {
		return err
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	companies := append([]model.Company(nil), store.companies...)
	companies[index] = company
	settingsIndex := -1
	for i := range store.settings {
		if store.settings[i].CompanyID == company.ID {
			settingsIndex = i
			break
		}
	}
	if settingsIndex < 0 {
		return fmt.Errorf("company settings not found")
	}
	settingsList := append([]model.CompanySettings(nil), store.settings...)
	settingsList[settingsIndex] = settings
	if err := store.write("companies.json", companies); err != nil {
		return err
	}
	if err := store.write("company_settings.json", settingsList); err != nil {
		if rollbackErr := store.write("companies.json", store.companies); rollbackErr != nil {
			return fmt.Errorf("write company settings: %w (also failed to restore company file: %v)", err, rollbackErr)
		}
		return err
	}
	store.companies, store.settings = companies, settingsList
	return nil
}

func (store *Store) write(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(store.dir, "."+name+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", name, err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("set permissions for %s: %w", name, err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary %s: %w", name, err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary %s: %w", name, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary %s: %w", name, err)
	}
	if err := os.Rename(tempName, filepath.Join(store.dir, name)); err != nil {
		return fmt.Errorf("replace %s: %w", name, err)
	}
	return nil
}

func loadJSON(path string, target any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return true, err
	}
	return true, nil
}

func companyExists(companies []model.Company, id string) bool {
	_, exists := findCompany(companies, id)
	return exists
}

func findCompany(companies []model.Company, id string) (model.Company, bool) {
	for _, company := range companies {
		if company.ID == id {
			return company, true
		}
	}
	return model.Company{}, false
}

func settingsForCompany(settings []model.CompanySettings, id string) *model.CompanySettings {
	for i := range settings {
		if settings[i].CompanyID == id {
			return &settings[i]
		}
	}
	return nil
}

func containsEquipmentModel(items []model.EquipmentModel, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsEquipment(items []model.Equipment, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasEquipmentForCompany(items []model.Equipment, companyID string) bool {
	for _, item := range items {
		if item.CompanyID == companyID {
			return true
		}
	}
	return false
}

func equipmentRecords(items []model.Equipment) []model.Equipment {
	result := append([]model.Equipment(nil), items...)
	for i := range result {
		result[i].EquipmentModel = model.EquipmentModel{}
	}
	return result
}

func containsMaterial(items []model.Material, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsStockBlank(items []model.StockBlank, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func companyMaterials(items []model.Material, companyID string) []model.Material {
	result := make([]model.Material, 0)
	for _, item := range items {
		if item.CompanyID == companyID {
			result = append(result, item)
		}
	}
	return result
}

func companyStockBlanks(items []model.StockBlank, companyID string) []model.StockBlank {
	result := make([]model.StockBlank, 0)
	for _, item := range items {
		if item.CompanyID == companyID {
			result = append(result, item)
		}
	}
	return result
}

func findEquipmentIndex(items []model.Equipment, companyID, id string) int {
	for i, item := range items {
		if item.ID == id {
			if item.CompanyID != companyID {
				return -2
			}
			return i
		}
	}
	return -1
}

func removeEquipment(items []model.Equipment, index int) []model.Equipment {
	return append(append([]model.Equipment(nil), items[:index]...), items[index+1:]...)
}

func findMaterialIndex(items []model.Material, companyID, id string) int {
	for i, item := range items {
		if item.ID == id {
			if item.CompanyID != companyID {
				return -2
			}
			return i
		}
	}
	return -1
}

func removeMaterial(items []model.Material, index int) []model.Material {
	return append(append([]model.Material(nil), items[:index]...), items[index+1:]...)
}

func findStockBlankIndex(items []model.StockBlank, companyID, id string) int {
	for i, item := range items {
		if item.ID == id {
			if item.CompanyID != companyID {
				return -2
			}
			return i
		}
	}
	return -1
}

func removeStockBlank(items []model.StockBlank, index int) []model.StockBlank {
	return append(append([]model.StockBlank(nil), items[:index]...), items[index+1:]...)
}

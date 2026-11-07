package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smetchic/application"
	"smetchic/model"
)

func TestMigrationExistsAndIncludesRequiredSchema(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "001_initial.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	text := string(migration)
	for _, expected := range []string{
		"CREATE TABLE companies",
		"CREATE TABLE users",
		"password_hash TEXT NOT NULL",
		"users_email_lower_uq",
		"CREATE TABLE company_memberships",
		"CREATE TABLE company_settings",
		"CREATE TABLE equipment_models",
		"CREATE TABLE equipment",
		"CREATE TABLE materials",
		"CREATE TABLE stock_blanks",
		"CREATE TABLE projects",
		"CREATE TABLE parts",
		"CREATE TABLE part_revisions",
		"CREATE TABLE estimates",
		"CREATE TABLE estimate_versions",
		"REFERENCES projects (company_id, id)",
		"REFERENCES materials (company_id, id)",
		"JSONB",
		"TIMESTAMPTZ",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("migration does not contain %q", expected)
		}
	}
}

func TestProjectLifecycleMigrationExistsAndPreservesVersionImmutability(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "002_project_lifecycle.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read project lifecycle migration: %v", err)
	}
	text := string(migration)
	for _, expected := range []string{
		"parts_company_project_name_uq",
		"estimates_company_revision_name_uq",
		"ADD COLUMN part_revision_id TEXT",
		"UPDATE estimate_versions",
		"estimate_versions_estimate_revision_fk",
		"CREATE TRIGGER estimate_versions_immutable",
		"CREATE TRIGGER estimate_versions_immutable_truncate",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("project lifecycle migration does not contain %q", expected)
		}
	}
}

func TestAuthMigrationAddsActiveFlagWithoutDuplicatingExistingAuthColumns(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "003_auth.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read auth migration: %v", err)
	}
	text := string(migration)
	for _, expected := range []string{
		"ALTER TABLE users",
		"ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("auth migration does not contain %q", expected)
		}
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		ok   bool
	}{
		{name: "postgres URL", dsn: "postgres://user:password@localhost:5432/cnc_estimator", ok: true},
		{name: "postgresql URL", dsn: "postgresql://user@localhost/cnc_estimator", ok: true},
		{name: "wrong scheme", dsn: "mysql://user@localhost/cnc_estimator", ok: false},
		{name: "missing database", dsn: "postgres://user@localhost", ok: false},
		{name: "malformed URL", dsn: "not a URL", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateURL(test.dsn)
			if (err == nil) != test.ok {
				t.Fatalf("validateURL() error = %v, want valid=%t", err, test.ok)
			}
		})
	}
}

func TestOpenDoesNotLeakConnectionString(t *testing.T) {
	dsn := "mysql://private-user:private-password@localhost/cnc_estimator"
	_, err := Open(context.Background(), dsn)
	if err == nil {
		t.Fatal("Open() accepted a non-PostgreSQL URL")
	}
	if strings.Contains(err.Error(), "private-password") || strings.Contains(err.Error(), "private-user") {
		t.Fatalf("error leaks connection credentials: %v", err)
	}
}

func TestPostgresIntegration(t *testing.T) {
	if os.Getenv(DatabaseEnvVar) == "" {
		t.Skipf("%s is not set", DatabaseEnvVar)
	}
	db, err := Open(context.Background(), os.Getenv(DatabaseEnvVar))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestProductionEconomicsMigrationContainsCostInputs(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "006_production_economics.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read production economics migration: %v", err)
	}
	text := string(migration)
	for _, expected := range []string{
		"CREATE TABLE company_economics",
		"CREATE TABLE equipment_economics",
		"CREATE TABLE personnel_rates",
		"CREATE TABLE company_overheads",
		"category TEXT NOT NULL CHECK (category IN ('utilities', 'other'))",
		"ADD COLUMN resource_minutes",
		"ADD COLUMN resource_units",
		"SELECT company_id, default_markup_percent, currency FROM company_settings",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("production economics migration does not contain %q", expected)
		}
	}
}

func TestPostgresLifecycleSurvivesReconnectAndEnforcesIsolation(t *testing.T) {
	databaseURL := os.Getenv(DatabaseEnvVar)
	if databaseURL == "" {
		t.Skipf("%s is not set", DatabaseEnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminDB, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	defer adminDB.Close()

	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate test schema name: %v", err)
	}
	schema := "lifecycle_test_" + hex.EncodeToString(random)
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create isolated integration schema: %v", err)
	}
	defer func() {
		if _, err := adminDB.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop isolated integration schema: %v", err)
		}
	}()

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("search_path", schema)
	query.Set("default_query_exec_mode", "simple_protocol")
	parsedURL.RawQuery = query.Encode()
	isolatedURL := parsedURL.String()

	db, err := Open(ctx, isolatedURL)
	if err != nil {
		t.Fatalf("open isolated integration connection: %v", err)
	}
	migrations := []string{
		"001_initial.sql", "002_project_lifecycle.sql", "003_auth.sql",
		"004_workspace.sql", "005_allow_multiple_orders_per_project.sql",
		"006_production_economics.sql",
	}
	for _, migrationName := range migrations {
		migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", migrationName))
		if err != nil {
			t.Fatalf("read %s: %v", migrationName, err)
		}
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s in isolated schema: %v", migrationName, err)
		}
	}

	repository := NewLifecycleRepository(db)
	service := application.NewService(repository, repository)
	company := model.Company{ID: "integration-company", Name: "Integration company"}
	settings := model.CompanySettings{CompanyID: company.ID, Currency: "RUB", DefaultMarkupPercent: 12.5}
	if err := service.EnsureCompany(ctx, company, settings); err != nil {
		t.Fatalf("create company and settings: %v", err)
	}
	authentication := application.NewAuthenticationService(repository)
	registeredUser, registeredCompany, err := authentication.RegisterCompany(
		ctx, "registered@example.test", "Registered owner", "abcdef", "Registered company",
	)
	if err != nil {
		t.Fatalf("register company and owner atomically: %v", err)
	}
	registeredRecord, err := repository.GetAuthUserByEmail(ctx, "REGISTERED@example.test")
	if err != nil || registeredRecord.User.ID != registeredUser.ID ||
		registeredRecord.PasswordHash == "abcdef" || !registeredRecord.Active {
		t.Fatalf("registered user = %#v, error = %v", registeredRecord, err)
	}
	registeredMemberships, err := repository.ListUserMemberships(ctx, registeredUser.ID)
	if err != nil || len(registeredMemberships) != 1 ||
		registeredMemberships[0].Company.ID != registeredCompany.ID ||
		registeredMemberships[0].Role != model.CompanyRoleOwner {
		t.Fatalf("registration memberships = %#v, error = %v", registeredMemberships, err)
	}
	if _, _, err := authentication.RegisterCompany(
		ctx, "REGISTERED@example.test", "Duplicate", "abcdef", "Must not persist",
	); !errors.Is(err, application.ErrEmailAlreadyExists) {
		t.Fatalf("duplicate registration error = %v", err)
	}
	if _, err := service.GetCompany(ctx, registeredCompany.ID); err != nil {
		t.Fatalf("registration company not persisted: %v", err)
	}
	authUser, err := authentication.CreateUser(
		ctx, "owner@example.test", "Test owner", "integration-password",
		company.ID, model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatalf("create authenticated user: %v", err)
	}
	if _, err := authentication.CreateUser(
		ctx, "OWNER@example.test", "Duplicate owner", "integration-password",
		company.ID, model.CompanyRoleOwner,
	); err == nil {
		t.Fatal("duplicate case-insensitive email was accepted")
	}
	if _, err := authentication.Login(ctx, "OWNER@example.test", "integration-password"); err != nil {
		t.Fatalf("login with persisted password hash: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET is_active = FALSE WHERE id = $1`, authUser.ID); err != nil {
		t.Fatalf("deactivate auth test user: %v", err)
	}
	if _, err := authentication.Login(ctx, "owner@example.test", "integration-password"); !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("inactive user login error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET is_active = TRUE WHERE id = $1`, authUser.ID); err != nil {
		t.Fatalf("reactivate auth test user: %v", err)
	}
	memberships, err := repository.ListUserMemberships(ctx, authUser.ID)
	if err != nil || len(memberships) != 1 || memberships[0].Company.ID != company.ID {
		t.Fatalf("user memberships = %#v, error = %v", memberships, err)
	}
	equipmentModel := model.EquipmentModel{
		ID: "integration-model", Type: model.EquipmentMilling,
		Manufacturer: "Test", Model: "Mill", MaxFeed: 5000,
	}
	if err := service.EnsureEquipmentModel(ctx, equipmentModel); err != nil {
		t.Fatalf("create equipment model: %v", err)
	}
	equipment := model.Equipment{
		ID: "integration-equipment", CompanyID: company.ID, EquipmentModelID: equipmentModel.ID,
		Name: "Integration mill", Status: model.EquipmentStatusActive, MachineRatePerHour: 1200,
	}
	if err := service.EnsureEquipment(ctx, equipment); err != nil {
		t.Fatalf("create equipment: %v", err)
	}
	if err := service.SaveCompanyEconomics(ctx, company.ID, model.CompanyEconomics{
		DefaultMarkupPercent: 18, MonthlyRent: 900, MonthlyProductionHours: 120,
		RentAllocationMethod: model.AllocationByProductionHours, Currency: "RUB",
	}); err != nil {
		t.Fatalf("save company production economics: %v", err)
	}
	if err := service.SaveEquipmentEconomics(ctx, company.ID, model.EquipmentEconomics{
		EquipmentID: equipment.ID, PurchasePrice: 1_000_000, ResidualValue: 100_000,
		UsefulLifeYears: 10, AnnualAvailableHours: 1800, ConsumptionKW: 8,
		ElectricityPricePerKWh: 7, AnnualMaintenanceCost: 20_000,
		AnnualRepairCost: 10_000, AnnualOtherCost: 5_000,
	}); err != nil {
		t.Fatalf("save equipment economics: %v", err)
	}
	if err := service.SavePersonnelRate(ctx, company.ID, model.PersonnelRate{
		Role: "operator", MonthlyCost: 80_000, MonthlyAvailableHours: 160, Currency: "RUB",
	}); err != nil {
		t.Fatalf("save personnel rate: %v", err)
	}
	if err := service.SaveCompanyOverhead(ctx, company.ID, model.CompanyOverhead{
		Name: "Utilities", Category: model.OverheadUtilities, Amount: 12_000,
		Period: model.OverheadMonthly, AllocationMethod: model.AllocationByProductionHours,
		Active: true, Currency: "RUB",
	}); err != nil {
		t.Fatalf("save company overhead: %v", err)
	}
	productionEconomics, err := service.ListProductionEconomics(ctx, company.ID)
	if err != nil || productionEconomics.Company.MonthlyRent != 900 ||
		len(productionEconomics.Equipment) != 1 || len(productionEconomics.Personnel) != 1 ||
		len(productionEconomics.Overheads) != 1 {
		t.Fatalf("production economics = %#v, error = %v", productionEconomics, err)
	}
	otherEconomics, err := service.ListProductionEconomics(ctx, registeredCompany.ID)
	if err != nil || len(otherEconomics.Equipment) != 0 ||
		len(otherEconomics.Personnel) != 0 || len(otherEconomics.Overheads) != 0 {
		t.Fatalf("other company production economics = %#v, error = %v", otherEconomics, err)
	}
	if loadedCompany, err := service.GetCompany(ctx, company.ID); err != nil || loadedCompany.Name != company.Name {
		t.Fatalf("read company = %#v, error = %v", loadedCompany, err)
	}

	project, err := service.CreateProject(ctx, company.ID, "Integration project", "persistent test")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	project, err = service.UpdateProject(ctx, company.ID, project.ID, "Updated integration project", "updated")
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "square.part"))
	if err != nil {
		t.Fatalf("read .part fixture: %v", err)
	}
	part, revision, err := service.UploadRevision(ctx, company.ID, project.ID, application.RevisionUpload{
		PartName: "Integration part", SourceFilename: "square.part", SourceContent: source,
	})
	if err != nil {
		t.Fatalf("create part and revision: %v", err)
	}
	estimate, version, err := service.Calculate(ctx, company.ID, revision.ID, "Integration estimate", equipment.ID, 12.5)
	if err != nil {
		t.Fatalf("create estimate and version: %v", err)
	}
	for label, snapshot := range map[string]json.RawMessage{
		"input": version.InputSnapshot, "settings": version.SettingsSnapshot, "result": version.ResultSnapshot,
	} {
		if !json.Valid(snapshot) {
			t.Errorf("%s snapshot is not valid JSON: %s", label, snapshot)
		}
	}
	var resultSnapshot map[string]json.RawMessage
	if err := json.Unmarshal(version.ResultSnapshot, &resultSnapshot); err != nil {
		t.Fatalf("decode result snapshot: %v", err)
	}
	if len(resultSnapshot["cost_breakdown"]) == 0 {
		t.Fatal("estimate result snapshot does not contain production cost breakdown")
	}
	originalVersion, err := service.GetEstimateVersion(ctx, company.ID, estimate.ID, version.ID)
	if err != nil {
		t.Fatalf("read persisted estimate snapshots before economics change: %v", err)
	}
	originalInput := append(json.RawMessage(nil), originalVersion.Version.InputSnapshot...)
	originalSettings := append(json.RawMessage(nil), originalVersion.Version.SettingsSnapshot...)
	originalResult := append(json.RawMessage(nil), originalVersion.Version.ResultSnapshot...)
	if err := service.SaveCompanyEconomics(ctx, company.ID, model.CompanyEconomics{
		DefaultMarkupPercent: 40, MonthlyRent: 4000, MonthlyProductionHours: 160,
		RentAllocationMethod: model.AllocationByProductionHours, Currency: "RUB",
	}); err != nil {
		t.Fatalf("change company economics after estimate: %v", err)
	}
	reloadedVersion, err := service.GetEstimateVersion(ctx, company.ID, estimate.ID, version.ID)
	if err != nil {
		t.Fatalf("read version after changing economics: %v", err)
	}
	if !bytes.Equal(reloadedVersion.Version.InputSnapshot, originalInput) ||
		!bytes.Equal(reloadedVersion.Version.SettingsSnapshot, originalSettings) ||
		!bytes.Equal(reloadedVersion.Version.ResultSnapshot, originalResult) {
		t.Fatal("historical estimate snapshots changed after company economics were updated")
	}

	otherCompany := model.Company{ID: "other-integration-company", Name: "Other company"}
	if err := service.EnsureCompany(ctx, otherCompany, model.CompanySettings{
		CompanyID: otherCompany.ID, Currency: "RUB",
	}); err != nil {
		t.Fatalf("create other company: %v", err)
	}
	companyUsers, err := service.ListCompanyUsers(ctx, company.ID)
	if err != nil || len(companyUsers) != 1 || companyUsers[0].User.ID != authUser.ID {
		t.Fatalf("company users = %#v, error = %v", companyUsers, err)
	}
	if _, err := service.SaveEquipment(ctx, company.ID, model.Equipment{
		ID: "workspace-integration-equipment", EquipmentModelID: equipmentModel.ID,
		Name: "Workspace mill", Status: model.EquipmentStatusActive, MachineRatePerHour: 1350,
	}); err != nil {
		t.Fatalf("save company equipment: %v", err)
	}
	if _, err := repository.GetEquipment(ctx, otherCompany.ID, "workspace-integration-equipment"); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company equipment lookup error = %v, want not found", err)
	}
	if _, err := service.SaveMaterial(ctx, company.ID, model.Material{
		ID: "workspace-integration-material", Name: "Aluminum", Grade: "6061",
		Category: model.MaterialCategoryAluminum, Density: 2700, PricePerKg: 500, Currency: "RUB",
	}); err != nil {
		t.Fatalf("save company material: %v", err)
	}
	companyMaterials, err := service.ListMaterials(ctx, company.ID)
	if err != nil || len(companyMaterials) != 1 || companyMaterials[0].ID != "workspace-integration-material" {
		t.Fatalf("company materials = %#v, error = %v", companyMaterials, err)
	}
	if _, err := service.SaveStockBlank(ctx, company.ID, model.StockBlank{
		ID: "workspace-integration-blank", MaterialID: "workspace-integration-material",
		Name: "Aluminum blank", Shape: model.StockBlankShapeRectangular,
		Length: 100, Width: 50, Height: 20, PricePerPiece: 1500, Currency: "RUB",
	}); err != nil {
		t.Fatalf("save company stock blank: %v", err)
	}
	companyStockBlanks, err := service.ListStockBlanks(ctx, company.ID)
	if err != nil || len(companyStockBlanks) != 1 || companyStockBlanks[0].PricePerPiece != 1500 {
		t.Fatalf("company stock blanks = %#v, error = %v", companyStockBlanks, err)
	}
	otherStockBlanks, err := service.ListStockBlanks(ctx, registeredCompany.ID)
	if err != nil || len(otherStockBlanks) != 0 {
		t.Fatalf("other company stock blanks = %#v, error = %v", otherStockBlanks, err)
	}
	otherMaterials, err := service.ListMaterials(ctx, otherCompany.ID)
	if err != nil || len(otherMaterials) != 0 {
		t.Fatalf("other company materials = %#v, error = %v", otherMaterials, err)
	}
	globalMaterials, err := service.ListGlobalMaterials(ctx)
	if err != nil || len(globalMaterials) == 0 {
		t.Fatalf("global materials = %#v, error = %v", globalMaterials, err)
	}
	if _, err := service.SaveConsumable(ctx, company.ID, model.Consumable{
		ID: "workspace-integration-cutter", Category: model.ConsumableCutter,
		Name: "Carbide end mill", DiameterMm: 10, FluteCount: 4, LengthMm: 60,
		StockQuantity: 3, UnitPrice: 1200, ResourceMinutes: 600, ResourceUnits: 200,
		ResourceUnitName: "cutting edges", Currency: "RUB", Attributes: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("save company consumable: %v", err)
	}
	companyConsumables, err := service.ListConsumables(ctx, company.ID)
	if err != nil || len(companyConsumables) != 1 || companyConsumables[0].ID != "workspace-integration-cutter" ||
		companyConsumables[0].ResourceMinutes != 600 || companyConsumables[0].ResourceUnits != 200 {
		t.Fatalf("company consumables = %#v, error = %v", companyConsumables, err)
	}
	otherConsumables, err := service.ListConsumables(ctx, otherCompany.ID)
	if err != nil || len(otherConsumables) != 0 {
		t.Fatalf("other company consumables = %#v, error = %v", otherConsumables, err)
	}
	firstOrder, err := service.CreateOrder(ctx, company.ID, authUser.ID, project.ID, "First order", "First")
	if err != nil {
		t.Fatalf("create company order: %v", err)
	}
	secondOrder, err := service.CreateOrder(ctx, company.ID, authUser.ID, project.ID, "Second order", "Second")
	if err != nil {
		t.Fatalf("create another order for the same project: %v", err)
	}
	firstOrder.Status = application.OrderStatusInProgress
	updatedOrder, err := service.UpdateOrder(ctx, firstOrder)
	if err != nil || updatedOrder.Status != application.OrderStatusInProgress {
		t.Fatalf("update company order = %#v, error = %v", updatedOrder, err)
	}
	companyOrders, err := service.ListOrders(ctx, company.ID)
	if err != nil || len(companyOrders) != 2 {
		t.Fatalf("company orders = %#v, error = %v", companyOrders, err)
	}
	if _, err := service.GetOrder(ctx, otherCompany.ID, firstOrder.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company order lookup error = %v, want not found", err)
	}
	if firstOrder.ID == secondOrder.ID {
		t.Fatal("two orders unexpectedly received the same ID")
	}
	if _, err := service.GetProject(ctx, otherCompany.ID, project.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company project lookup error = %v, want not found", err)
	}
	if _, err := service.GetRevisionEstimate(ctx, otherCompany.ID, revision.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company revision lookup error = %v, want not found", err)
	}
	if _, err := service.GetEstimate(ctx, otherCompany.ID, estimate.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company estimate lookup error = %v, want not found", err)
	}
	if _, err := repository.GetEquipment(ctx, otherCompany.ID, equipment.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-company equipment lookup error = %v, want not found", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close first application connection: %v", err)
	}
	db, err = Open(ctx, isolatedURL)
	if err != nil {
		t.Fatalf("reopen PostgreSQL connection: %v", err)
	}
	defer db.Close()
	repository = NewLifecycleRepository(db)
	service = application.NewService(repository, repository)

	loadedProject, err := service.GetProject(ctx, company.ID, project.ID)
	if err != nil || loadedProject.Name != "Updated integration project" || loadedProject.Description != "updated" {
		t.Fatalf("project after reconnect = %#v, error = %v", loadedProject, err)
	}
	loadedCompany, err := service.GetCompany(ctx, company.ID)
	if err != nil || loadedCompany.Name != company.Name {
		t.Fatalf("company after reconnect = %#v, error = %v", loadedCompany, err)
	}
	persistedUser, err := repository.GetAuthUserByID(ctx, authUser.ID)
	if err != nil || persistedUser.User.Email != authUser.Email || !persistedUser.Active ||
		persistedUser.PasswordHash == "integration-password" {
		t.Fatalf("user after reconnect = %#v, error = %v", persistedUser, err)
	}
	loadedSettings, err := service.GetCompanySettings(ctx, company.ID)
	if err != nil || loadedSettings.Currency != settings.Currency {
		t.Fatalf("settings after reconnect = %#v, error = %v", loadedSettings, err)
	}
	loadedEquipment, err := repository.GetEquipment(ctx, company.ID, equipment.ID)
	if err != nil || loadedEquipment.MachineRatePerHour != equipment.MachineRatePerHour {
		t.Fatalf("equipment after reconnect = %#v, error = %v", loadedEquipment, err)
	}
	loadedPart, err := service.GetPart(ctx, company.ID, part.ID)
	if err != nil || loadedPart.ProjectID != project.ID {
		t.Fatalf("part after reconnect = %#v, error = %v", loadedPart, err)
	}
	revisions, err := service.ListPartRevisions(ctx, company.ID, part.ID)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("revisions after reconnect = %#v, error = %v", revisions, err)
	}
	if revisions[0].SourceContent != string(source) || revisions[0].SourceHash != revision.SourceHash {
		t.Fatal("source content or hash changed after reconnect")
	}
	detail, err := service.GetRevisionEstimate(ctx, company.ID, revision.ID)
	if err != nil || detail.Revision.PartID != part.ID || detail.Project.ID != project.ID {
		t.Fatalf("revision linkage after reconnect = %#v, error = %v", detail, err)
	}
	loadedEstimate, err := service.GetEstimate(ctx, company.ID, estimate.ID)
	if err != nil || loadedEstimate.PartRevisionID != revision.ID {
		t.Fatalf("estimate after reconnect = %#v, error = %v", loadedEstimate, err)
	}
	loadedVersion, err := service.GetEstimateVersion(ctx, company.ID, estimate.ID, version.ID)
	if err != nil || loadedVersion.Version.VersionNumber != 1 ||
		!json.Valid(loadedVersion.Version.ResultSnapshot) {
		t.Fatalf("estimate version after reconnect = %#v, error = %v", loadedVersion, err)
	}
	if _, err := db.ExecContext(ctx, `
			UPDATE estimate_versions SET final_price = final_price + 1
			WHERE company_id = $1 AND id = $2`, company.ID, version.ID); err == nil {
		t.Fatal("estimate version UPDATE succeeded despite immutable trigger")
	}
	if _, err := db.ExecContext(ctx, `
			DELETE FROM estimate_versions WHERE company_id = $1 AND id = $2`,
		company.ID, version.ID); err == nil {
		t.Fatal("estimate version DELETE succeeded despite immutable trigger")
	}
}

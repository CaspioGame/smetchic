package postgres

import (
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
	migrations := []string{"001_initial.sql", "002_project_lifecycle.sql"}
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

	otherCompany := model.Company{ID: "other-integration-company", Name: "Other company"}
	if err := service.EnsureCompany(ctx, otherCompany, model.CompanySettings{
		CompanyID: otherCompany.ID, Currency: "RUB",
	}); err != nil {
		t.Fatalf("create other company: %v", err)
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

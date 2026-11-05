package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

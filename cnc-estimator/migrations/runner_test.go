package migrations

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"smetchic/storage/postgres"
)

func TestMigrationFilesAreOrderedAndVersioned(t *testing.T) {
	items, err := readMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("migration count = %d, want 5", len(items))
	}
	for index, item := range items {
		if item.version != index+1 {
			t.Errorf("migration at index %d has version %d", index, item.version)
		}
	}
}

func TestReadMigrationsRejectsDuplicateVersions(t *testing.T) {
	files := fstestMap(map[string]string{
		"001_initial.sql":   "SELECT 1",
		"001_duplicate.sql": "SELECT 2",
	})
	if _, err := readMigrations(files); err == nil {
		t.Fatal("duplicate migration versions were accepted")
	}
}

func TestMigrationRunnerOnEmptyAndExistingDatabases(t *testing.T) {
	db := openIsolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	failures := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := Apply(ctx, db.DB, &bytes.Buffer{})
			failures <- err
		}()
	}
	for range 2 {
		if err := <-failures; err != nil {
			t.Fatalf("concurrent migrations failed: %v", err)
		}
	}
	assertMigrationHistory(t, ctx, db, 5)

	var output bytes.Buffer
	results, err := Apply(ctx, db.DB, &output)
	if err != nil {
		t.Fatalf("re-run migrations: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("re-run results count = %d, want 5", len(results))
	}
	for _, result := range results {
		if result.Status != "already applied" {
			t.Errorf("re-run migration %03d status = %q", result.Version, result.Status)
		}
	}
	if !strings.Contains(output.String(), "Database schema is up to date") {
		t.Fatalf("re-run output does not report up-to-date schema: %s", output.String())
	}
}

func TestMigrationRunnerBootstrapsManuallyAppliedSchema(t *testing.T) {
	db := openIsolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	items, err := readMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.version == 3 || item.version == 5 {
			continue
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin manual migration %03d: %v", item.version, err)
		}
		script, err := stripTransactionWrapper(item.sql)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("prepare manual migration %03d: %v", item.version, err)
		}
		if err := txConnectionExec(ctx, conn, script); err != nil {
			_ = tx.Rollback()
			t.Fatalf("apply migration %03d manually: %v", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit manual migration %03d: %v", item.version, err)
		}
		if item.version == 1 {
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO companies (id, name) VALUES ('preserved-company', 'Preserved company')`); err != nil {
				t.Fatalf("insert pre-existing company data: %v", err)
			}
		}
	}
	conn.Close()

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE orders
		ADD CONSTRAINT orders_company_id_project_id_key UNIQUE (company_id, project_id)`); err != nil {
		t.Fatalf("simulate older workspace migration constraint: %v", err)
	}
	results, err := Apply(ctx, db.DB, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("bootstrap manually migrated database: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("bootstrap result count = %d, want 5", len(results))
	}
	for _, result := range results {
		if (result.Version == 3 || result.Version == 5) && result.Status != "applied" {
			t.Errorf("missing migration %03d status = %q, want applied", result.Version, result.Status)
		}
		if result.Version != 3 && result.Version != 5 && result.Status != "already applied" {
			t.Errorf("pre-existing migration %03d status = %q", result.Version, result.Status)
		}
	}
	assertMigrationHistory(t, ctx, db, 5)

	var companyName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM companies WHERE id = 'preserved-company'`).Scan(&companyName); err != nil {
		t.Fatalf("read preserved company: %v", err)
	}
	if companyName != "Preserved company" {
		t.Fatalf("preserved company name = %q", companyName)
	}
	var oldConstraintExists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conrelid = to_regclass('orders')
			  AND conname = 'orders_company_id_project_id_key'
		)`).Scan(&oldConstraintExists); err != nil {
		t.Fatal(err)
	}
	if oldConstraintExists {
		t.Fatal("005 migration did not drop the old one-order-per-project constraint")
	}
}

func TestMigrationFailureRollsBackSchemaAndHistory(t *testing.T) {
	db := openIsolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	files := fstestMap(map[string]string{
		"001_failure.sql": `
			CREATE TABLE should_rollback (id INTEGER);
			SELECT 1 / 0;
		`,
	})
	if _, err := ApplyFS(ctx, db.DB, files, &bytes.Buffer{}); err == nil {
		t.Fatal("failing migration unexpectedly succeeded")
	}
	var tableExists bool
	if err := db.QueryRowContext(ctx, `
		SELECT to_regclass('should_rollback') IS NOT NULL`).Scan(&tableExists); err != nil {
		t.Fatal(err)
	}
	if tableExists {
		t.Fatal("failed migration left a partial table behind")
	}
	assertMigrationHistory(t, ctx, db, 0)
}

func TestMigrationRunnerRefusesPartialLegacySchema(t *testing.T) {
	db := openIsolatedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `CREATE TABLE companies (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, db.DB, &bytes.Buffer{}); err == nil ||
		!strings.Contains(err.Error(), "partial schema footprint") {
		t.Fatalf("partial legacy schema error = %v", err)
	}
	var companyTableExists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('companies') IS NOT NULL`).Scan(&companyTableExists); err != nil {
		t.Fatal(err)
	}
	if !companyTableExists {
		t.Fatal("runner changed the pre-existing partial schema")
	}
	assertMigrationHistory(t, ctx, db, 0)
}

func openIsolatedDatabase(t *testing.T) *postgres.DB {
	t.Helper()
	databaseURL := os.Getenv(postgres.DatabaseEnvVar)
	if databaseURL == "" {
		t.Skipf("%s is not set", postgres.DatabaseEnvVar)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}

	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		admin.Close()
		t.Fatalf("generate isolated schema name: %v", err)
	}
	schema := "migration_test_" + hex.EncodeToString(random)
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("create isolated schema: %v", err)
	}
	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatalf("parse test database URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("search_path", schema)
	query.Set("default_query_exec_mode", "simple_protocol")
	parsedURL.RawQuery = query.Encode()
	db, err := postgres.Open(ctx, parsedURL.String())
	if err != nil {
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
		t.Fatalf("open isolated schema connection: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop isolated test schema: %v", err)
		}
		if err := admin.Close(); err != nil {
			t.Errorf("close PostgreSQL admin connection: %v", err)
		}
	})
	return db
}

func assertMigrationHistory(t *testing.T, ctx context.Context, db *postgres.DB, want int) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migration history: %v", err)
	}
	if count != want {
		t.Fatalf("migration history count = %d, want %d", count, want)
	}
}

func fstestMap(entries map[string]string) fs.FS {
	files := make(fstest.MapFS, len(entries))
	for filename, content := range entries {
		files[path.Clean(filename)] = &fstest.MapFile{Data: []byte(content)}
	}
	return files
}

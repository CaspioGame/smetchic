package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed *.sql
var migrationFiles embed.FS

const (
	advisoryLockClass int32 = 0x434e43
	advisoryLockID    int32 = 0x455354
)

var migrationFilename = regexp.MustCompile(`^([0-9]+)_([a-zA-Z0-9_]+)\.sql$`)

type Result struct {
	Version int
	Name    string
	Status  string
}

type migration struct {
	version int
	name    string
	sql     string
}

type migrationState int

const (
	migrationAbsent migrationState = iota
	migrationPresent
	migrationPartial
)

func Apply(ctx context.Context, db *sql.DB, output io.Writer) ([]Result, error) {
	return ApplyFS(ctx, db, migrationFiles, output)
}

func ApplyFS(ctx context.Context, db *sql.DB, files fs.FS, output io.Writer) ([]Result, error) {
	if db == nil {
		return nil, fmt.Errorf("PostgreSQL database is required")
	}
	if output == nil {
		output = io.Discard
	}
	migrations, err := readMigrations(files)
	if err != nil {
		return nil, err
	}

	connection, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Close()

	if _, err := connection.ExecContext(ctx, `SELECT pg_advisory_lock($1, $2)`, advisoryLockClass, advisoryLockID); err != nil {
		return nil, fmt.Errorf("acquire PostgreSQL migration lock: %w", err)
	}
	defer func() {
		if _, err := connection.ExecContext(context.Background(),
			`SELECT pg_advisory_unlock($1, $2)`, advisoryLockClass, advisoryLockID); err != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not release PostgreSQL migration lock: %v\n", err)
		}
	}()

	if _, err := connection.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations table: %w", err)
	}

	applied, err := readApplied(ctx, connection)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprintln(output, "Running database migrations...")
	results := make([]Result, 0, len(migrations))
	for _, item := range migrations {
		if recordedName, ok := applied[item.version]; ok {
			if recordedName != item.name {
				return results, fmt.Errorf(
					"migration version %03d is recorded as %q, but file is %q",
					item.version, recordedName, item.name,
				)
			}
			result := Result{Version: item.version, Name: item.name, Status: "already applied"}
			results = append(results, result)
			_, _ = fmt.Fprintf(output, "Migration %03d_%s: already applied\n", item.version, item.name)
			continue
		}

		state, err := detectMigrationState(ctx, connection, item.version)
		if err != nil {
			return results, fmt.Errorf("inspect migration %03d_%s: %w", item.version, item.name, err)
		}
		switch state {
		case migrationPresent:
			if err := recordMigration(ctx, connection, item); err != nil {
				return results, fmt.Errorf("record existing migration %03d_%s: %w", item.version, item.name, err)
			}
			applied[item.version] = item.name
			result := Result{Version: item.version, Name: item.name, Status: "already applied"}
			results = append(results, result)
			_, _ = fmt.Fprintf(output, "Migration %03d_%s: already applied (existing schema detected)\n", item.version, item.name)
		case migrationPartial:
			return results, fmt.Errorf(
				"migration %03d_%s has a partial schema footprint; refusing to re-run it automatically",
				item.version, item.name,
			)
		case migrationAbsent:
			if err := applyMigration(ctx, connection, item); err != nil {
				return results, fmt.Errorf("apply migration %03d_%s: %w", item.version, item.name, err)
			}
			applied[item.version] = item.name
			result := Result{Version: item.version, Name: item.name, Status: "applied"}
			results = append(results, result)
			_, _ = fmt.Fprintf(output, "Migration %03d_%s: applied\n", item.version, item.name)
		}
	}
	_, _ = fmt.Fprintln(output, "Database schema is up to date")
	return results, nil
}

func readMigrations(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("find migration files: %w", err)
	}
	migrations := make([]migration, 0, len(names))
	versions := make(map[int]string, len(names))
	for _, filename := range names {
		match := migrationFilename.FindStringSubmatch(path.Base(filename))
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %q; expected NNN_name.sql", filename)
		}
		version, err := strconv.Atoi(match[1])
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", filename)
		}
		name := match[2]
		if previous, ok := versions[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %03d in %q and %q", version, previous, filename)
		}
		contents, err := fs.ReadFile(files, filename)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", filename, err)
		}
		if strings.TrimSpace(string(contents)) == "" {
			return nil, fmt.Errorf("migration %q is empty", filename)
		}
		versions[version] = filename
		migrations = append(migrations, migration{version: version, name: name, sql: string(contents)})
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no PostgreSQL migration files found")
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func readApplied(ctx context.Context, connection *sql.Conn) (map[int]string, error) {
	rows, err := connection.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}
	return applied, nil
}

func detectMigrationState(ctx context.Context, connection *sql.Conn, version int) (migrationState, error) {
	switch version {
	case 1:
		var count int
		err := connection.QueryRowContext(ctx, `
			SELECT count(*) FROM information_schema.tables
			WHERE table_schema = ANY(current_schemas(false))
			  AND table_name = ANY($1)`,
			[]string{
				"companies", "users", "company_memberships", "company_settings",
				"equipment_models", "equipment", "materials", "stock_blanks", "projects",
				"parts", "part_revisions", "estimates", "estimate_versions",
			}).Scan(&count)
		if err != nil {
			return migrationAbsent, err
		}
		return stateFromCount(count, 13), nil
	case 2:
		var found int
		err := connection.QueryRowContext(ctx, `
			SELECT
				(to_regclass('parts_company_project_name_uq') IS NOT NULL)::int
				+ (to_regclass('estimates_company_revision_name_uq') IS NOT NULL)::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'estimate_versions'
					  AND column_name = 'part_revision_id'
				))::int
				+ (EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conrelid = to_regclass('estimate_versions')
					  AND conname = 'estimate_versions_estimate_revision_fk'
				))::int`).Scan(&found)
		if err != nil {
			return migrationAbsent, err
		}
		return stateFromCount(found, 4), nil
	case 3:
		var exists bool
		err := connection.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = ANY(current_schemas(false))
				  AND table_name = 'users'
				  AND column_name = 'is_active'
			)`).Scan(&exists)
		if err != nil {
			return migrationAbsent, err
		}
		if exists {
			return migrationPresent, nil
		}
		return migrationAbsent, nil
	case 4:
		var count int
		err := connection.QueryRowContext(ctx, `
			SELECT count(*) FROM information_schema.tables
			WHERE table_schema = ANY(current_schemas(false))
			  AND table_name = ANY($1)`,
			[]string{"global_materials", "consumables", "orders"}).Scan(&count)
		if err != nil {
			return migrationAbsent, err
		}
		return stateFromCount(count, 3), nil
	case 5:
		var tableExists, oldConstraintExists bool
		err := connection.QueryRowContext(ctx, `
			SELECT to_regclass('orders') IS NOT NULL,
			       EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conrelid = to_regclass('orders')
					  AND conname = 'orders_company_id_project_id_key'
			       )`).Scan(&tableExists, &oldConstraintExists)
		if err != nil {
			return migrationAbsent, err
		}
		if !tableExists || oldConstraintExists {
			return migrationAbsent, nil
		}
		return migrationPresent, nil
	case 6:
		var count int
		err := connection.QueryRowContext(ctx, `
			SELECT
				(EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'company_economics'
				))::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'equipment_economics'
				))::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'personnel_rates'
				))::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'company_overheads'
				))::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'consumables'
					  AND column_name = 'resource_minutes'
				))::int
				+ (EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema = ANY(current_schemas(false))
					  AND table_name = 'company_overheads'
					  AND column_name = 'category'
				))::int`).Scan(&count)
		if err != nil {
			return migrationAbsent, err
		}
		return stateFromCount(count, 6), nil
	default:
		return migrationAbsent, nil
	}
}

func stateFromCount(count, expected int) migrationState {
	switch {
	case count == 0:
		return migrationAbsent
	case count == expected:
		return migrationPresent
	default:
		return migrationPartial
	}
}

func recordMigration(ctx context.Context, connection *sql.Conn, item migration) error {
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration history transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		item.version, item.name); err != nil {
		return fmt.Errorf("insert migration history row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration history transaction: %w", err)
	}
	return nil
}

func applyMigration(ctx context.Context, connection *sql.Conn, item migration) error {
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	script, err := stripTransactionWrapper(item.sql)
	if err != nil {
		return fmt.Errorf("prepare SQL: %w", err)
	}
	err = txConnectionExec(ctx, connection, script)
	if err != nil {
		return fmt.Errorf("execute SQL: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		item.version, item.name); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func stripTransactionWrapper(script string) (string, error) {
	trimmed := strings.TrimSpace(script)
	upper := strings.ToUpper(trimmed)
	startsWithBegin := strings.HasPrefix(upper, "BEGIN;")
	endsWithCommit := strings.HasSuffix(upper, "COMMIT;")
	if startsWithBegin != endsWithCommit {
		return "", fmt.Errorf("migration must have both BEGIN and COMMIT wrappers or neither")
	}
	if !startsWithBegin {
		return script, nil
	}
	trimmed = strings.TrimSpace(trimmed[len("BEGIN;") : len(trimmed)-len("COMMIT;")])
	if trimmed == "" {
		return "", fmt.Errorf("migration transaction is empty")
	}
	return trimmed, nil
}

func txConnectionExec(ctx context.Context, connection *sql.Conn, query string) error {
	return connection.Raw(func(driverConnection any) error {
		pgxConnection, ok := driverConnection.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("unexpected PostgreSQL driver connection %T", driverConnection)
		}
		_, err := pgxConnection.Conn().PgConn().Exec(ctx, query).ReadAll()
		return err
	})
}

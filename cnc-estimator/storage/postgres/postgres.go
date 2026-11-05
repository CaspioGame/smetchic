package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	DriverName     = "pgx"
	DatabaseEnvVar = "DATABASE_URL"
)

type DB struct {
	*sql.DB
}

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("%s is required", DatabaseEnvVar)
	}
	if err := validateURL(databaseURL); err != nil {
		return nil, err
	}

	connection, err := sql.Open(DriverName, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	if err := connection.PingContext(ctx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &DB{DB: connection}, nil
}

func OpenFromEnv(ctx context.Context) (*DB, error) {
	return Open(ctx, os.Getenv(DatabaseEnvVar))
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("PostgreSQL database is not open")
	}
	return db.DB.PingContext(ctx)
}

func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func validateURL(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("invalid %s", DatabaseEnvVar)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("%s must use the postgres or postgresql URL scheme", DatabaseEnvVar)
	}
	if parsed.Hostname() == "" || strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("%s must include a host and database name", DatabaseEnvVar)
	}
	return nil
}

func DefaultPingTimeout() time.Duration {
	return 5 * time.Second
}

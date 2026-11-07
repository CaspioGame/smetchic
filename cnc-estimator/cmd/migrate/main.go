package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"smetchic/migrations"
	"smetchic/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := postgres.OpenFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	fmt.Println("Database connected")
	_, err = migrations.Apply(ctx, db.DB, os.Stdout)
	if err != nil {
		return fmt.Errorf("database migrations failed: %w", err)
	}
	return nil
}

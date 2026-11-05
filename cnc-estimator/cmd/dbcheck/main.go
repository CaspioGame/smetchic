package main

import (
	"context"
	"fmt"
	"os"

	"smetchic/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout())
	defer cancel()

	db, err := postgres.OpenFromEnv(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL ping failed: %w", err)
	}
	fmt.Println("PostgreSQL connection successful.")
	return nil
}

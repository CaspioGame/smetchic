package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"smetchic/application"
	"smetchic/equipment"
	"smetchic/estimator"
	"smetchic/migrations"
	"smetchic/model"
	"smetchic/output"
	"smetchic/parser"
	"smetchic/storage/postgres"
	"smetchic/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout())
		db, err := postgres.OpenFromEnv(ctx)
		cancel()
		if err != nil {
			return fmt.Errorf("не удалось открыть PostgreSQL (задайте DATABASE_URL): %w", err)
		}
		log.Printf("Database connected")
		defer func() {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("не удалось закрыть соединение PostgreSQL: %v", closeErr)
			}
		}()
		migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err = migrations.Apply(migrationCtx, db.DB, log.Writer())
		migrationCancel()
		if err != nil {
			return fmt.Errorf("не удалось применить миграции PostgreSQL: %w", err)
		}
		repository := postgres.NewLifecycleRepository(db)
		service := application.NewService(repository, repository)
		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout())
		_, err = service.InitializeDemoCompany(bootstrapCtx)
		bootstrapCancel()
		if err != nil {
			return fmt.Errorf("не удалось подготовить компанию PostgreSQL: %w", err)
		}

		authentication := application.NewAuthenticationService(repository)
		handler, err := web.NewAuthenticatedLifecycleCabinetHandler(nil, service, authentication)
		if err != nil {
			return fmt.Errorf("не удалось подготовить Web-интерфейс: %w", err)
		}
		address := os.Getenv("APP_ADDR")
		if address == "" {
			address = "127.0.0.1:8080"
		}
		log.Printf("Server started on %s", address)
		return http.ListenAndServe(address, handler)
	}
	if len(args) != 1 {
		return fmt.Errorf("Использование:\n  go run .                 запустить Web-интерфейс\n  go run main.go <file.part>  выполнить расчёт в CLI")
	}

	catalog, err := equipment.DefaultCatalog()
	if err != nil {
		return err
	}
	equipmentItem, err := catalog.GetByID("dekart-s4040")
	if err != nil {
		return err
	}
	settings := model.EstimateSettings{EquipmentID: equipmentItem.ID, MarkupPercent: 50}

	part, err := parser.ParseFile(args[0])
	if err != nil {
		return err
	}

	estimate, err := estimator.Calculate(part, equipmentItem, settings)
	if err != nil {
		return err
	}
	output.PrintEstimate(estimate)
	return nil
}

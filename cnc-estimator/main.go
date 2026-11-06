package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"smetchic/application"
	"smetchic/equipment"
	"smetchic/estimator"
	"smetchic/model"
	"smetchic/output"
	"smetchic/parser"
	"smetchic/storage"
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
		defer func() {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("не удалось закрыть соединение PostgreSQL: %v", closeErr)
			}
		}()
		repository := postgres.NewLifecycleRepository(db)
		service := application.NewService(repository, repository)
		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout())
		company, err := service.InitializeDemoCompany(bootstrapCtx)
		bootstrapCancel()
		if err != nil {
			return fmt.Errorf("не удалось подготовить компанию PostgreSQL: %w", err)
		}

		var store *storage.Store
		store, err = storage.NewStore("data")
		if err != nil {
			log.Printf("legacy JSON cabinet is unavailable; PostgreSQL lifecycle remains enabled: %v", err)
			store = nil
		}
		handler, err := web.NewLifecycleCabinetHandler(store, company, service)
		if err != nil {
			return fmt.Errorf("не удалось подготовить Web-интерфейс: %w", err)
		}
		address := "127.0.0.1:8080"
		log.Printf("CNC Estimator доступен по адресу http://localhost:8080")
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

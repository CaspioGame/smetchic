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
		store, err := storage.NewStore("data")
		if err != nil {
			return fmt.Errorf("не удалось открыть JSON-хранилище: %w", err)
		}
		companyID, err := store.FirstCompanyID()
		if err != nil {
			return fmt.Errorf("не удалось выбрать компанию: %w", err)
		}
		var handler http.Handler
		if os.Getenv(postgres.DatabaseEnvVar) == "" {
			handler, err = web.NewCabinetHandler(store, companyID)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), postgres.DefaultPingTimeout())
			db, openErr := postgres.OpenFromEnv(ctx)
			cancel()
			if openErr != nil {
				return fmt.Errorf("не удалось подключиться к PostgreSQL: %w", openErr)
			}
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					log.Printf("не удалось закрыть соединение PostgreSQL: %v", closeErr)
				}
			}()
			snapshot, snapshotErr := store.Snapshot(companyID)
			if snapshotErr != nil {
				return fmt.Errorf("не удалось загрузить демонстрационную компанию: %w", snapshotErr)
			}
			repository := postgres.NewLifecycleRepository(db)
			service := application.NewService(repository, application.NewJSONConfigProvider(store))
			bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = service.EnsureCompany(bootstrapCtx, snapshot.Company, snapshot.Settings)
			bootstrapCancel()
			if err != nil {
				return fmt.Errorf("не удалось подготовить компанию в PostgreSQL: %w", err)
			}
			handler, err = web.NewLifecycleCabinetHandler(store, companyID, service)
		}
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

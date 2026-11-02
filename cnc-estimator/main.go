package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"smetchic/equipment"
	"smetchic/estimator"
	"smetchic/model"
	"smetchic/output"
	"smetchic/parser"
	"smetchic/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	catalog, err := equipment.DefaultCatalog()
	if err != nil {
		return err
	}
	equipmentItem, err := catalog.GetByID("dekart-s4040")
	if err != nil {
		return err
	}
	settings := model.EstimateSettings{
		EquipmentID:   equipmentItem.ID,
		MarkupPercent: 50,
	}

	if len(args) == 0 {
		handler, err := web.NewHandler(catalog, settings)
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

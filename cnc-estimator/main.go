package main

import "fmt"

func main() {

	machine := Machine{
		ID:           "dekart-s4040",
		Name:         "DeKart S4040",
		WorkX:        400,
		WorkY:        400,
		WorkZ:        200,
		SpindlePower: 1.5,
		MaxRPM:       24000,
		MaxFeed:      5000,
		HourlyRate:   2500,
	}

	tool := Tool{
		ID:       "endmill-8",
		Name:     "Концевая фреза",
		Diameter: 8,
		Teeth:    2,
		MaxRPM:   24000,
	}

	operation := Operation{
		Name: "Черновая обработка полости",

		Tool: tool,

		Width:  200,
		Length: 200,
		Depth:  20,

		Feed:     1200,
		StepDown: 2,
		StepOver: 4,
	}

	result := CalculatePocket(
		operation,
		machine.HourlyRate,
	)

	fmt.Println("=== CNC ESTIMATOR ===")
	fmt.Println()

	fmt.Printf("Станок: %s\n", machine.Name)
	fmt.Printf("Фреза: Ø%.1f мм\n", tool.Diameter)
	fmt.Println()

	fmt.Println("ПОЛОСТЬ")
	fmt.Printf(
		"Размер: %.0f × %.0f × %.0f мм\n",
		operation.Width,
		operation.Length,
		operation.Depth,
	)

	fmt.Println()

	fmt.Println("ПАРАМЕТРЫ")
	fmt.Printf("Подача: %.0f мм/мин\n", operation.Feed)
	fmt.Printf("Stepdown: %.1f мм\n", operation.StepDown)
	fmt.Printf("Stepover: %.1f мм\n", operation.StepOver)

	fmt.Println()

	fmt.Println("РАСЧЁТ")
	fmt.Printf("Проходов по Z: %d\n", result.PassesZ)
	fmt.Printf("Проходов по XY: %d\n", result.PassesXY)
	fmt.Printf(
		"Длина траектории: %.0f мм\n",
		result.CuttingLength,
	)

	fmt.Printf(
		"Время резания: %.2f мин\n",
		result.TimeMinutes,
	)

	fmt.Printf(
		"Стоимость резания: %.2f ₽\n",
		result.Cost,
	)
}

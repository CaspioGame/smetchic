package main

import "math"

type Operation struct {
	Name string
	Tool Tool

	// Размер полости
	Width  float64
	Length float64
	Depth  float64

	// Параметры обработки
	Feed     float64
	StepDown float64
	StepOver float64
}

type OperationResult struct {
	PassesZ       int
	PassesXY      int
	CuttingLength float64
	TimeMinutes   float64
	Cost          float64
}

// CalculatePocket считает приблизительную траекторию прямоугольной полости.
func CalculatePocket(
	operation Operation,
	hourlyRate float64,
) OperationResult {

	if operation.Tool.Diameter <= 0 ||
		operation.StepDown <= 0 ||
		operation.StepOver <= 0 ||
		operation.Feed <= 0 {
		return OperationResult{}
	}

	// -----------------------------
	// 1. Количество проходов по Z
	// -----------------------------

	passesZ := int(math.Ceil(
		operation.Depth / operation.StepDown,
	))

	// -----------------------------
	// 2. Рабочая ширина полости
	// -----------------------------

	// Фреза должна полностью находиться
	// внутри полости.
	workingWidth := operation.Width - operation.Tool.Diameter

	workingLength := operation.Length - operation.Tool.Diameter

	if workingWidth <= 0 || workingLength <= 0 {
		return OperationResult{}
	}

	// -----------------------------
	// 3. Количество проходов по XY
	// -----------------------------

	passesXY := int(math.Ceil(
		workingWidth / operation.StepOver,
	))

	// Минимум один проход
	if passesXY < 1 {
		passesXY = 1
	}

	// -----------------------------
	// 4. Длина траектории XY
	// -----------------------------

	// Упрощённая модель:
	//
	// каждый проход идёт практически
	// по всей длине полости.

	cuttingLengthPerZ := float64(passesXY) * workingLength

	// Учитываем все уровни Z.
	cuttingLength := cuttingLengthPerZ * float64(passesZ)

	// -----------------------------
	// 5. Время резания
	// -----------------------------

	timeMinutes := CalculateCuttingTime(
		cuttingLength,
		operation.Feed,
	)

	// -----------------------------
	// 6. Стоимость
	// -----------------------------

	cost := CalculateOperationCost(
		timeMinutes,
		hourlyRate,
	)

	return OperationResult{
		PassesZ:       passesZ,
		PassesXY:      passesXY,
		CuttingLength: cuttingLength,
		TimeMinutes:   timeMinutes,
		Cost:          cost,
	}
}

// CalculateCuttingTime считает время резания в минутах.
func CalculateCuttingTime(
	lengthMM float64,
	feedMMMin float64,
) float64 {

	if feedMMMin <= 0 {
		return 0
	}

	return lengthMM / feedMMMin
}

// CalculateOperationCost считает стоимость операции.
func CalculateOperationCost(
	timeMinutes float64,
	hourlyRate float64,
) float64 {

	return timeMinutes / 60 * hourlyRate
}

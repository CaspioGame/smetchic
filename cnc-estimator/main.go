package main

import (
	"fmt"
	"os"

	"smetchic/calculator"
	"smetchic/model"
	"smetchic/output"
	"smetchic/parser"
	"smetchic/planner"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Usage:\ngo run main.go <path-to-part-file>")
	}

	part, err := parser.ParseFile(args[0])
	if err != nil {
		return err
	}

	machine := model.Machine{
		Name:       "DeKart S4040",
		HourlyRate: 1500,
		MaxFeed:    5000,
	}

	operations, err := planner.BuildOperations(part, machine)
	if err != nil {
		return err
	}
	totalMachineTime := calculator.CalculateOperationTimes(operations)
	machineCost, totalCost, suggestedPrice := calculator.CalculateCosts(totalMachineTime, machine)

	estimate := model.Estimate{
		Part:             part,
		Operations:       operations,
		TotalMachineTime: totalMachineTime,
		MachineCost:      machineCost,
		TotalCost:        totalCost,
		SuggestedPrice:   suggestedPrice,
		Machine:          machine,
	}
	output.PrintEstimate(estimate)
	return nil
}

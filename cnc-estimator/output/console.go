package output

import (
	"fmt"

	"smetchic/model"
)

func PrintEstimate(estimate model.Estimate) {
	fmt.Println()
	fmt.Println("PART")
	fmt.Println(estimate.Part.SourceFile)
	fmt.Printf("Material: %s\n", estimate.Part.Material)
	fmt.Printf("Stock: %.0f x %.0f x %.0f mm\n", estimate.Part.Stock.Width, estimate.Part.Stock.Length, estimate.Part.Stock.Height)

	fmt.Println()
	fmt.Println("FEATURES")
	featureNames := make(map[string]string, len(estimate.Part.Features))
	typeCounts := make(map[model.FeatureType]int)
	for i, feature := range estimate.Part.Features {
		typeCounts[feature.Type]++
		name := featureDisplayName(feature.Type, typeCounts[feature.Type])
		featureNames[feature.ID] = name
		fmt.Printf("%d. %s\n   %s\n", i+1, name, describeFeature(feature))
	}

	fmt.Println()
	fmt.Println("EQUIPMENT")
	fmt.Println(estimate.Equipment.Name)
	fmt.Printf("Rate: %.2f ₽/hour\n", estimate.Equipment.MachineRatePerHour)
	fmt.Printf("Maximum feed: %.0f mm/min\n", estimate.Equipment.EquipmentModel.MaxFeed)

	fmt.Println()
	fmt.Println("OPERATIONS")
	for i, operation := range estimate.Operations {
		fmt.Printf("%d. %s — %s\n", i+1, featureNames[operation.FeatureID], operation.Name)
		fmt.Printf("   ID: %s | FeatureID: %s | Strategy: %s\n", operation.ID, operation.FeatureID, operation.Strategy)
		fmt.Printf("   Equipment: %s | Tool: %s Ø%.0f mm\n", operation.Equipment.Name, operation.Tool.Name, operation.Tool.Diameter)
		fmt.Printf("   Feed: %.0f mm/min | RPM: %.0f\n", operation.CuttingParameters.Feed, operation.CuttingParameters.RPM)
		fmt.Printf("   Depth: %.2f mm | StepDown: %.2f mm | StepOver: %.2f mm\n",
			operation.CuttingParameters.Depth, operation.CuttingParameters.StepDown, operation.CuttingParameters.StepOver)
		fmt.Printf("   Estimated/test toolpath length: %.0f mm\n", operation.EstimatedToolpathLength)
	}

	fmt.Println()
	fmt.Println("TIME")
	for i, operation := range estimate.Operations {
		fmt.Printf("%d. %s — %s: %.2f min (based on estimated/test toolpath length: %.0f mm)\n",
			i+1, featureNames[operation.FeatureID], operation.Name, operation.TimeMinutes, operation.EstimatedToolpathLength)
	}
	fmt.Printf("Total machine time: %.2f min\n", estimate.TotalMachineTime)

	fmt.Println()
	fmt.Println("COST")
	fmt.Printf("Machine cost: %.2f ₽\n", estimate.MachineCost)
	fmt.Printf("Total cost: %.2f ₽\n", estimate.TotalCost)

	fmt.Println()
	fmt.Println("ESTIMATE")
	fmt.Printf("Suggested price (demo markup): %.2f ₽\n", estimate.SuggestedPrice)
}

func featureDisplayName(featureType model.FeatureType, index int) string {
	names := map[model.FeatureType]string{
		model.FeaturePocket:  "Pocket",
		model.FeatureHole:    "Hole",
		model.FeatureContour: "Contour",
		model.FeatureChamfer: "Chamfer",
	}
	name := names[featureType]
	if featureType == model.FeatureContour && index == 1 {
		return name
	}
	return fmt.Sprintf("%s #%d", name, index)
}

func describeFeature(feature model.Feature) string {
	var description string
	switch feature.Type {
	case model.FeaturePocket, model.FeatureContour:
		description = fmt.Sprintf("%.0f x %.0f x %.0f mm", feature.Width, feature.Length, feature.Depth)
	case model.FeatureHole:
		description = fmt.Sprintf("Ø%.0f x %.0f mm", feature.Diameter, feature.Depth)
	case model.FeatureChamfer:
		description = fmt.Sprintf("%.1f mm chamfer", feature.ChamferSize)
	}
	if feature.Tolerance > 0 {
		description += fmt.Sprintf(" (tolerance ±%.2f mm)", feature.Tolerance)
	}
	return description
}

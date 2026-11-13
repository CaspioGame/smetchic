package application

import (
	"fmt"
	"slices"
)

const (
	ModuleStockMaterial   = "stock_material"
	ModuleCatalogMaterial = "catalog_material"
	ModuleStockBlank      = "stock_blank"
	ModuleStockTool       = "stock_tool"
	ModuleCatalogTool     = "catalog_tool"
	ModuleOperator        = "operator"
	ModuleSetup           = "setup"
	ModuleProgramming     = "programming"
)

type EstimateModule struct {
	ID, Name, Group string
	Fields          []string
}

func EstimateModuleCatalog() []EstimateModule {
	return []EstimateModule{
		{ModuleStockMaterial, "Материал со склада", "material", []string{"material_id", "material_weight_kg"}},
		{ModuleCatalogMaterial, "Материал из общей базы", "material", []string{"global_material_id", "material_weight_kg"}},
		{ModuleStockBlank, "Заготовка со склада", "material", []string{"stock_blank_id"}},
		{ModuleStockTool, "Инструмент со склада", "tool", []string{"consumable_id", "tool_usage_minutes", "tool_usage_reason"}},
		{ModuleCatalogTool, "Инструмент из общей базы", "tool", []string{"global_tool_id", "tool_usage_minutes", "tool_usage_reason"}},
		{ModuleOperator, "Оператор", "", []string{"fixed_operator_cost"}},
		{ModuleSetup, "Наладчик", "", []string{"setup_time_minutes", "setup_reason", "fixed_setup_cost"}},
		{ModuleProgramming, "Программист", "", []string{"programming_hours", "programming_reason", "fixed_programming_cost"}},
	}
}

type EstimateModules []string

func (modules EstimateModules) Has(id string) bool {
	return slices.Contains(modules, id)
}

func (modules EstimateModules) Validate() error {
	seen, groups := map[string]bool{}, map[string]bool{}
	catalog := EstimateModuleCatalog()
	for _, id := range modules {
		index := slices.IndexFunc(catalog, func(module EstimateModule) bool { return module.ID == id })
		if index < 0 || seen[id] {
			return fmt.Errorf("проверьте выбранные модули сметы")
		}
		group := catalog[index].Group
		if group != "" && groups[group] {
			return fmt.Errorf("выберите один источник материала и один источник инструмента")
		}
		seen[id], groups[group] = true, true
	}
	return nil
}

func (input EstimateInput) ActiveModules() EstimateModules {
	if input.Modules != nil {
		return slices.Clone(input.Modules)
	}
	// Older estimates always included the operator rate.
	modules := EstimateModules{ModuleOperator}
	if input.StockBlankID != "" {
		modules = append(modules, ModuleStockBlank)
	} else if input.MaterialID != "" {
		modules = append(modules, ModuleStockMaterial)
	} else if input.GlobalMaterialID != "" {
		modules = append(modules, ModuleCatalogMaterial)
	}
	if input.ConsumableID != "" {
		modules = append(modules, ModuleStockTool)
	} else if input.GlobalToolID != "" {
		modules = append(modules, ModuleCatalogTool)
	}
	if input.SetupTimeMinutes > 0 || input.SetupReason != "" || (input.ProjectLabor != nil && input.ProjectLabor.Setup != nil) {
		modules = append(modules, ModuleSetup)
	}
	if input.ProgrammingHours > 0 || input.ProgrammingReason != "" || (input.ProjectLabor != nil && input.ProjectLabor.Programming != nil) {
		modules = append(modules, ModuleProgramming)
	}
	return modules
}

func (input *EstimateInput) clearInactiveModules() {
	modules := input.Modules
	if modules == nil {
		return
	}
	if !modules.Has(ModuleStockBlank) {
		input.StockBlankID, input.StockBlankSource = "", ""
	}
	if !modules.Has(ModuleStockMaterial) {
		input.MaterialID = ""
	}
	if !modules.Has(ModuleCatalogMaterial) {
		input.GlobalMaterialID = ""
	}
	if !modules.Has(ModuleStockMaterial) && !modules.Has(ModuleCatalogMaterial) {
		input.MaterialWeightKg, input.MaterialSource = 0, ""
	}
	if !modules.Has(ModuleStockTool) {
		input.ConsumableID = ""
	}
	if !modules.Has(ModuleCatalogTool) {
		input.GlobalToolID = ""
	}
	if !modules.Has(ModuleStockTool) && !modules.Has(ModuleCatalogTool) {
		input.ToolUsageMinutes, input.ToolUsageSource, input.ToolUsageReason, input.ToolSource = 0, "", "", ""
	}
	if !modules.Has(ModuleSetup) {
		input.SetupTimeMinutes, input.SetupSource, input.SetupReason = 0, "", ""
	}
	if !modules.Has(ModuleProgramming) {
		input.ProgrammingHours, input.ProgrammingSource, input.ProgrammingReason = 0, "", ""
	}
	if input.ProjectLabor != nil {
		labor := *input.ProjectLabor
		if !modules.Has(ModuleOperator) {
			labor.Operator = nil
		}
		if !modules.Has(ModuleSetup) {
			labor.Setup = nil
		}
		if !modules.Has(ModuleProgramming) {
			labor.Programming = nil
		}
		input.ProjectLabor = nil
		if labor.Operator != nil || labor.Setup != nil || labor.Programming != nil {
			input.ProjectLabor = &labor
		}
	}
}

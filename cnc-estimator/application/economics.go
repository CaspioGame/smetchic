package application

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"smetchic/costing"
	"smetchic/estimator"
	"smetchic/model"
)

type ProductionEstimateOptions struct {
	EstimateID           string
	DocumentInput        EstimateInput
	EstimateWorkingInput json.RawMessage
	OrderID              string
	Quantity             int
	SetupTimeMinutes     float64
	ProgrammingHours     float64
	MaterialID           string
	GlobalMaterialID     string
	StockBlankID         string
	ConsumableID         string
	ToolUsageMinutes     float64
	MachineTimeOverride  *EstimateOverride
}

type EconomicsRepository interface {
	GetCompanyEconomics(context.Context, string) (model.CompanyEconomics, error)
	SaveCompanyEconomics(context.Context, model.CompanyEconomics) error
	ListEquipmentEconomics(context.Context, string) ([]model.EquipmentEconomics, error)
	GetEquipmentEconomics(context.Context, string, string) (*model.EquipmentEconomics, error)
	SaveEquipmentEconomics(context.Context, model.EquipmentEconomics) error
	DeleteEquipmentEconomics(context.Context, string, string) error
	ListPersonnelRates(context.Context, string) ([]model.PersonnelRate, error)
	SavePersonnelRate(context.Context, model.PersonnelRate) error
	DeletePersonnelRate(context.Context, string, string) error
	ListCompanyOverheads(context.Context, string) ([]model.CompanyOverhead, error)
	SaveCompanyOverhead(context.Context, model.CompanyOverhead) error
	DeleteCompanyOverhead(context.Context, string, string) error
	ListStockBlanks(context.Context, string) ([]model.StockBlank, error)
}

type StockBlankRepository interface {
	SaveStockBlank(context.Context, model.StockBlank) error
	DeleteStockBlank(context.Context, string, string) error
}

type CompanyProductionEconomics struct {
	Company   model.CompanyEconomics
	Equipment []model.EquipmentEconomics
	Personnel []model.PersonnelRate
	Overheads []model.CompanyOverhead
}

func (service *Service) ListStockBlanks(ctx context.Context, companyID string) ([]model.StockBlank, error) {
	repository, err := service.economicsRepository()
	if err != nil {
		return nil, err
	}
	return repository.ListStockBlanks(ctx, companyID)
}

func (service *Service) SaveStockBlank(ctx context.Context, companyID string, item model.StockBlank) (model.StockBlank, error) {
	repository, ok := service.repository.(StockBlankRepository)
	if !ok {
		return model.StockBlank{}, fmt.Errorf("repository does not support stock blanks")
	}
	if item.ID == "" {
		id, err := newID("stock-blank")
		if err != nil {
			return model.StockBlank{}, err
		}
		item.ID = id
	}
	item.CompanyID = companyID
	materials, err := service.repository.ListMaterials(ctx, companyID)
	if err != nil {
		return model.StockBlank{}, err
	}
	var selectedMaterial *model.Material
	for index := range materials {
		if materials[index].ID == item.MaterialID {
			selectedMaterial = &materials[index]
			break
		}
	}
	if selectedMaterial == nil {
		return model.StockBlank{}, Invalid("Выберите материал этой компании.", nil)
	}
	if err := item.ValidateForMaterial(*selectedMaterial); err != nil {
		return model.StockBlank{}, Invalid("Проверьте размеры и цену заготовки.", err)
	}
	if item.Currency != selectedMaterial.Currency {
		return model.StockBlank{}, Invalid("Валюта заготовки должна совпадать с валютой материала.", nil)
	}
	if err := repository.SaveStockBlank(ctx, item); err != nil {
		return model.StockBlank{}, err
	}
	return item, nil
}

func (service *Service) DeleteStockBlank(ctx context.Context, companyID, stockBlankID string) error {
	repository, ok := service.repository.(StockBlankRepository)
	if !ok {
		return fmt.Errorf("repository does not support stock blanks")
	}
	return repository.DeleteStockBlank(ctx, companyID, stockBlankID)
}

func (service *Service) ListProductionEconomics(ctx context.Context, companyID string) (CompanyProductionEconomics, error) {
	repository, err := service.economicsRepository()
	if err != nil {
		return CompanyProductionEconomics{}, err
	}
	result := CompanyProductionEconomics{}
	result.Company, err = repository.GetCompanyEconomics(ctx, companyID)
	if err != nil {
		return CompanyProductionEconomics{}, err
	}
	result.Equipment, err = repository.ListEquipmentEconomics(ctx, companyID)
	if err != nil {
		return CompanyProductionEconomics{}, err
	}
	result.Personnel, err = repository.ListPersonnelRates(ctx, companyID)
	if err != nil {
		return CompanyProductionEconomics{}, err
	}
	result.Overheads, err = repository.ListCompanyOverheads(ctx, companyID)
	if err != nil {
		return CompanyProductionEconomics{}, err
	}
	return result, nil
}

func (service *Service) SaveCompanyEconomics(ctx context.Context, companyID string, item model.CompanyEconomics) error {
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return Invalid("Проверьте экономические настройки компании.", err)
	}
	return service.updateCompanyEconomics(ctx, item)
}

func (service *Service) SaveEquipmentEconomics(ctx context.Context, companyID string, item model.EquipmentEconomics) error {
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return Invalid("Проверьте экономические параметры станка.", err)
	}
	if _, err := service.config.GetEquipment(ctx, companyID, item.EquipmentID); err != nil {
		return err
	}
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.SaveEquipmentEconomics(ctx, item)
}

func (service *Service) GetEquipmentEconomics(ctx context.Context, companyID, equipmentID string) (*model.EquipmentEconomics, error) {
	repository, err := service.economicsRepository()
	if err != nil {
		return nil, err
	}
	return repository.GetEquipmentEconomics(ctx, companyID, equipmentID)
}

func (service *Service) SavePersonnelRate(ctx context.Context, companyID string, item model.PersonnelRate) error {
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return Invalid("Проверьте ставку персонала.", err)
	}
	if item.ID == "" {
		id, err := newID("personnel-rate")
		if err != nil {
			return err
		}
		item.ID = id
	}
	return service.savePersonnelRate(ctx, item)
}

func (service *Service) DeleteEquipmentEconomics(ctx context.Context, companyID, equipmentID string) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.DeleteEquipmentEconomics(ctx, companyID, equipmentID)
}

func (service *Service) DeletePersonnelRate(ctx context.Context, companyID, rateID string) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.DeletePersonnelRate(ctx, companyID, rateID)
}

func (service *Service) SaveCompanyOverhead(ctx context.Context, companyID string, item model.CompanyOverhead) error {
	item.CompanyID = companyID
	if item.ID == "" {
		id, err := newID("overhead")
		if err != nil {
			return err
		}
		item.ID = id
	}
	if err := item.Validate(); err != nil {
		return Invalid("Проверьте накладные расходы.", err)
	}
	return service.saveCompanyOverhead(ctx, item)
}

func (service *Service) DeleteCompanyOverhead(ctx context.Context, companyID, overheadID string) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.DeleteCompanyOverhead(ctx, companyID, overheadID)
}

func (service *Service) economicsRepository() (EconomicsRepository, error) {
	repository, ok := service.repository.(EconomicsRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support production economics")
	}
	return repository, nil
}

func (service *Service) CalculateWithProductionEconomics(
	ctx context.Context,
	companyID, revisionID, estimateName, equipmentID string,
	markup float64,
	options ProductionEstimateOptions,
) (EstimateRecord, EstimateVersionRecord, error) {
	if math.IsNaN(markup) || math.IsInf(markup, 0) || markup < 0 {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Наценка должна быть конечным неотрицательным числом.", nil)
	}
	if options.Quantity <= 0 {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Количество деталей должно быть больше нуля.", nil)
	}
	for name, value := range map[string]float64{
		"setup time": options.SetupTimeMinutes, "programming time": options.ProgrammingHours,
		"tool usage": options.ToolUsageMinutes,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Проверьте параметры производственного расчёта.", fmt.Errorf("%s must be non-negative", name))
		}
	}
	if options.DocumentInput.Name == "" {
		options.DocumentInput = EstimateInput{
			Name: estimateName, PartRevisionID: revisionID, Quantity: options.Quantity,
			EquipmentID: equipmentID, MaterialID: options.MaterialID,
			GlobalMaterialID: options.GlobalMaterialID, StockBlankID: options.StockBlankID,
			ConsumableID: options.ConsumableID, ToolUsageMinutes: options.ToolUsageMinutes,
			SetupTimeMinutes: options.SetupTimeMinutes, ProgrammingHours: options.ProgrammingHours,
			MarkupPercent: markup, MarkupSource: EstimateSourceCompanyData,
			MarkupReason: "",
		}
		if options.MachineTimeOverride != nil {
			override := *options.MachineTimeOverride
			options.DocumentInput.MachineTime = &override
		}
	}
	if options.DocumentInput.MarkupSource == "" {
		options.DocumentInput.MarkupSource = EstimateSourceCompanyData
	}
	if len(options.EstimateWorkingInput) == 0 {
		workingInput, marshalErr := json.Marshal(options.DocumentInput)
		if marshalErr != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize estimate working input: %w", marshalErr)
		}
		options.EstimateWorkingInput = workingInput
	}
	detail, err := service.repository.GetRevisionEstimate(ctx, companyID, revisionID)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	equipment, err := service.config.GetEquipment(ctx, companyID, equipmentID)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("не удалось получить станок: %w", err)
	}
	if equipment.Status != model.EquipmentStatusActive {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Для расчёта можно выбрать только активный станок.", nil)
	}
	settings, err := service.config.GetCompanySettings(ctx, companyID)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("не удалось получить настройки компании: %w", err)
	}
	if err := settings.Validate(); err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("некорректные настройки компании: %w", err)
	}
	if settings.CompanyID != companyID {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("настройки компании не соответствуют текущей компании")
	}

	result := model.Estimate{
		Part: detail.Revision.ParsedPart, Equipment: equipment,
		Settings: model.EstimateSettings{EquipmentID: equipment.ID, MarkupPercent: markup},
	}
	var automaticMachineTimeMinutes, usedMachineTimeMinutes float64
	machineTimeSource := EstimateSourceAutomatic
	machineTimeReason := ""
	if options.MachineTimeOverride == nil {
		if detail.Revision.ParserVersion == attachmentVersion {
			return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Введите время обработки одной детали.", nil)
		}
		result, err = estimator.Calculate(detail.Revision.ParsedPart, equipment, result.Settings)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("расчёт не выполнен: %w", err)
		}
		automaticMachineTimeMinutes = result.TotalMachineTime
		usedMachineTimeMinutes = automaticMachineTimeMinutes
	} else {
		override := options.MachineTimeOverride
		if override.Value < 0 || math.IsNaN(override.Value) || math.IsInf(override.Value, 0) {
			return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Машинное время должно быть конечным и неотрицательным.", nil)
		}
		if override.Source != EstimateSourceUserOverride && override.Source != EstimateSourceAIOverride {
			return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Укажите источник времени обработки.", nil)
		}
		usedMachineTimeMinutes = override.Value * 60
		machineTimeSource = override.Source
		machineTimeReason = strings.TrimSpace(override.Reason)
	}
	options.EstimateWorkingInput, err = json.Marshal(options.DocumentInput)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize normalized estimate input: %w", err)
	}
	companyEconomics := model.CompanyEconomics{
		CompanyID: companyID, DefaultMarkupPercent: settings.DefaultMarkupPercent,
		RentAllocationMethod: model.AllocationByProductionHours, Currency: settings.Currency,
	}
	var equipmentEconomics *model.EquipmentEconomics
	personnel := make([]model.PersonnelRate, 0)
	overheads := make([]model.CompanyOverhead, 0)
	if economicsRepository, ok := service.repository.(EconomicsRepository); ok {
		companyEconomics, err = economicsRepository.GetCompanyEconomics(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load company production economics: %w", err)
		}
		equipmentEconomics, err = economicsRepository.GetEquipmentEconomics(ctx, companyID, equipmentID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load equipment economics: %w", err)
		}
		personnel, err = economicsRepository.ListPersonnelRates(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load personnel rates: %w", err)
		}
		overheads, err = economicsRepository.ListCompanyOverheads(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load company overheads: %w", err)
		}
	}
	if companyEconomics.Currency != settings.Currency {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("company production economics currency does not match company settings")
	}
	var operatorRate, programmingRate *model.PersonnelRate
	var setupRate float64
	for index := range personnel {
		rate := &personnel[index]
		if rate.Currency != settings.Currency {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("personnel rate %q has a different currency", rate.Role)
		}
		switch rate.Role {
		case "operator":
			operatorRate = rate
		case "programmer":
			programmingRate = rate
		case "setup":
			if rate.MonthlyAvailableHours > 0 {
				setupRate = rate.MonthlyCost / rate.MonthlyAvailableHours
			}
		}
	}
	for _, overhead := range overheads {
		if overhead.Currency != settings.Currency {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("overhead %q has a different currency", overhead.Name)
		}
	}

	if options.StockBlankID != "" && (options.MaterialID != "" || options.GlobalMaterialID != "") ||
		options.MaterialID != "" && options.GlobalMaterialID != "" {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Выберите только один источник стоимости материала.", nil)
	}
	materialCost, materialSource := 0.0, "not_selected"
	stock := detail.Revision.ParsedPart.Stock
	materialWeight := options.DocumentInput.MaterialWeightKg
	materialCostByWeight := func(density, price float64) (float64, error) {
		weight := materialWeight
		if weight == 0 && stock.Width > 0 && stock.Length > 0 && stock.Height > 0 {
			weight = stock.Width * stock.Length * stock.Height * density / 1_000_000_000
		}
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return 0, Invalid("Укажите массу заготовки на одну деталь в килограммах.", nil)
		}
		return weight * price, nil
	}
	var materialSnapshot *model.Material
	var globalMaterialSnapshot *model.GlobalMaterial
	var stockBlankSnapshot *model.StockBlank
	if strings.TrimSpace(options.StockBlankID) != "" {
		economicsRepository, ok := service.repository.(EconomicsRepository)
		if !ok {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("repository does not support stock blanks")
		}
		stockBlanks, err := economicsRepository.ListStockBlanks(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load stock blanks: %w", err)
		}
		var blank *model.StockBlank
		for index := range stockBlanks {
			if stockBlanks[index].ID == options.StockBlankID {
				blank = &stockBlanks[index]
				break
			}
		}
		if blank == nil {
			return EstimateRecord{}, EstimateVersionRecord{}, ErrNotFound
		}
		materials, err := service.repository.ListMaterials(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load stock blank material: %w", err)
		}
		for index := range materials {
			if materials[index].ID != blank.MaterialID {
				continue
			}
			material := materials[index]
			if blank.Currency != settings.Currency || material.Currency != settings.Currency {
				return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("selected stock blank uses a different currency")
			}
			materialCost, err = blank.MaterialCost(material)
			if err != nil {
				return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Не удалось рассчитать стоимость выбранной заготовки.", err)
			}
			materialSnapshot = &material
			stockBlankSnapshot = blank
			materialSource = "company_stock_blank"
			break
		}
		if materialSnapshot == nil {
			return EstimateRecord{}, EstimateVersionRecord{}, ErrNotFound
		}
	} else if strings.TrimSpace(options.MaterialID) != "" {
		materials, err := service.repository.ListMaterials(ctx, companyID)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load company materials: %w", err)
		}
		for index := range materials {
			if materials[index].ID != options.MaterialID {
				continue
			}
			material := materials[index]
			if material.Currency != settings.Currency {
				return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("selected material uses a different currency")
			}
			materialCost, err = materialCostByWeight(material.Density, material.PricePerKg)
			if err != nil {
				return EstimateRecord{}, EstimateVersionRecord{}, err
			}
			materialSource = "company_material_price_per_kg"
			materialSnapshot = &material
			break
		}
		if materialSnapshot == nil {
			return EstimateRecord{}, EstimateVersionRecord{}, ErrNotFound
		}
	} else if strings.TrimSpace(options.GlobalMaterialID) != "" {
		items, err := service.repository.ListGlobalMaterials(ctx)
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load global reference materials: %w", err)
		}
		for index := range items {
			if items[index].ID != options.GlobalMaterialID {
				continue
			}
			material := items[index]
			if material.Currency != settings.Currency {
				return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("selected reference material uses a different currency")
			}
			materialCost, err = materialCostByWeight(material.Density, material.ReferencePriceKg)
			if err != nil {
				return EstimateRecord{}, EstimateVersionRecord{}, err
			}
			materialSource = "global_reference_price"
			globalMaterialSnapshot = &material
			break
		}
		if globalMaterialSnapshot == nil {
			return EstimateRecord{}, EstimateVersionRecord{}, ErrNotFound
		}
	}

	tools := make([]costing.ToolUse, 0, 1)
	var consumableSnapshot *model.Consumable
	toolID := strings.TrimSpace(options.ConsumableID)
	globalToolID := strings.TrimSpace(options.DocumentInput.GlobalToolID)
	if toolID != "" && globalToolID != "" {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Выберите инструмент только из одного источника.", nil)
	}
	if toolID != "" || globalToolID != "" {
		var items []model.Consumable
		if globalToolID != "" {
			toolID = globalToolID
			items, err = service.ListGlobalTools(ctx)
		} else {
			items, err = service.repository.ListConsumables(ctx, companyID)
		}
		if err != nil {
			return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("load company tooling: %w", err)
		}
		for index := range items {
			if items[index].ID != toolID {
				continue
			}
			item := items[index]
			if item.Currency != settings.Currency {
				return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("selected tool uses a different currency")
			}
			consumableSnapshot = &item
			tools = append(tools, costing.ToolUse{
				Name: item.Name, PurchasePrice: item.UnitPrice,
				ResourceMinutes: item.ResourceMinutes, UsedMinutes: options.ToolUsageMinutes,
			})
			break
		}
		if consumableSnapshot == nil {
			return EstimateRecord{}, EstimateVersionRecord{}, ErrNotFound
		}
	}

	calculationEconomics := companyEconomics
	calculationEconomics.DefaultMarkupPercent = markup
	breakdown, err := costing.Calculate(costing.Input{
		Quantity: options.Quantity, Currency: settings.Currency,
		MachineTimeMinutes:       usedMachineTimeMinutes,
		LegacyMachineRatePerHour: equipment.MachineRatePerHour,
		EquipmentEconomics:       equipmentEconomics, OperatorRate: operatorRate,
		ProgrammingRate: programmingRate, ProgrammingHours: options.ProgrammingHours,
		SetupTimeMinutes: options.SetupTimeMinutes, SetupCostPerHour: setupRate,
		ProjectLabor:         options.DocumentInput.ProjectLabor,
		ExcludeOperator:      options.DocumentInput.Modules != nil && !options.DocumentInput.Modules.Has(ModuleOperator),
		MaterialCostPerPiece: materialCost, MaterialPriceSource: materialSource,
		Tools: tools, CompanyEconomics: calculationEconomics, Overheads: overheads,
	})
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("calculate production cost: %w", err)
	}
	if machineTimeSource != EstimateSourceAutomatic {
		breakdown.Warnings = append(breakdown.Warnings,
			fmt.Sprintf("Machine time manually overridden: %s", machineTimeReason))
	}
	modules := options.DocumentInput.Modules
	if materialSource == "not_selected" && (modules == nil || modules.Has(ModuleStockBlank) || modules.Has(ModuleStockMaterial) || modules.Has(ModuleCatalogMaterial)) {
		breakdown.Warnings = append(breakdown.Warnings, "Material price is not selected; material cost is excluded.")
	}
	if materialSource == "global_reference_price" {
		breakdown.Warnings = append(breakdown.Warnings, "Global material price is a reference estimate, not a supplier quotation.")
	}
	if consumableSnapshot == nil && (modules == nil || modules.Has(ModuleStockTool) || modules.Has(ModuleCatalogTool)) {
		breakdown.Warnings = append(breakdown.Warnings, "Tooling is not selected; tooling cost is excluded.")
	} else if consumableSnapshot != nil && globalToolID != "" && consumableSnapshot.ResourceMinutes == 0 && options.ToolUsageMinutes == 0 {
		breakdown.Warnings = append(breakdown.Warnings,
			fmt.Sprintf("Tool %q resource is not configured; tooling cost is excluded.", consumableSnapshot.Name))
	}
	if consumableSnapshot != nil && options.ToolUsageMinutes > 0 {
		breakdown.ToolSupply = planToolSupply(*consumableSnapshot, options.ToolUsageMinutes*float64(options.Quantity), globalToolID == "")
	}
	result.CostBreakdown = &breakdown
	result.TotalMachineTime = usedMachineTimeMinutes
	result.MachineCost = breakdown.MachineCost
	result.TotalCost = breakdown.TotalCost
	result.SuggestedPrice = breakdown.SellingPrice

	inputSnapshot, err := json.Marshal(struct {
		Project                   ProjectRecord
		Part                      PartRecord
		Revision                  PartRevisionRecord
		WorkingInput              EstimateInput
		Options                   ProductionEstimateOptions
		AutomaticMachineTimeHours float64
		UsedMachineTimeHours      float64
		MachineTimeSource         EstimateValueSource
		MachineTimeReason         string
		MarkupSource              EstimateValueSource
		MarkupReason              string
	}{
		detail.Project, detail.Part, detail.Revision, options.DocumentInput, options,
		automaticMachineTimeMinutes / 60, usedMachineTimeMinutes / 60,
		machineTimeSource, machineTimeReason,
		options.DocumentInput.MarkupSource, options.DocumentInput.MarkupReason,
	})
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize estimate input: %w", err)
	}
	settingsSnapshot, err := json.Marshal(struct {
		CompanySettings    model.CompanySettings
		CompanyEconomics   model.CompanyEconomics
		Equipment          model.Equipment
		EquipmentEconomics *model.EquipmentEconomics
		PersonnelRates     []model.PersonnelRate
		Overheads          []model.CompanyOverhead
		Material           *model.Material
		GlobalMaterial     *model.GlobalMaterial
		StockBlank         *model.StockBlank
		Consumable         *model.Consumable
		EstimateSettings   model.EstimateSettings
	}{settings, companyEconomics, equipment, equipmentEconomics, personnel, overheads,
		materialSnapshot, globalMaterialSnapshot, stockBlankSnapshot, consumableSnapshot, result.Settings})
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize estimate settings: %w", err)
	}
	resultSnapshot, err := json.Marshal(result)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize estimate result: %w", err)
	}
	estimateName = strings.TrimSpace(estimateName)
	if estimateName == "" {
		estimateName = "Расчёт"
	}
	estimateID, err := newID("estimate")
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	versionID, err := newID("estimate-version")
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	estimate := EstimateRecord{
		ID: options.EstimateID, CompanyID: companyID, PartID: detail.Part.ID,
		ProjectID: detail.Project.ID, OrderID: options.OrderID,
		PartRevisionID: revisionID, Name: estimateName, Quantity: options.Quantity,
		Status: EstimateStatusDraft, WorkingInput: options.EstimateWorkingInput,
	}
	if estimate.ID == "" {
		estimate.ID = estimateID
	}
	version := EstimateVersionRecord{
		ID: versionID, CompanyID: companyID, PartRevisionID: revisionID,
		EquipmentID: equipment.ID, EquipmentModelID: equipment.EquipmentModelID,
		InputSnapshot: inputSnapshot, SettingsSnapshot: settingsSnapshot, ResultSnapshot: resultSnapshot,
		MachineTimeHours: usedMachineTimeMinutes * float64(options.Quantity) / 60,
		HourlyRate:       equipment.MachineRatePerHour,
		MachineCost:      breakdown.MachineCost,
		MarkupPercent:    markup,
		FinalPrice:       breakdown.SellingPrice,
		Currency:         settings.Currency,
	}
	return service.repository.CreateEstimateVersion(ctx, estimate, version)
}

func planToolSupply(tool model.Consumable, requiredMinutes float64, companyStock bool) *model.ToolSupply {
	plan := &model.ToolSupply{RequiredMinutes: requiredMinutes}
	if !companyStock || tool.ResourceMinutes <= 0 ||
		(tool.StockQuantity > 0 && len(tool.UsedMinutesByUnit) != tool.StockQuantity) {
		return plan
	}
	plan.StockKnown = true
	for _, used := range tool.UsedMinutesByUnit {
		plan.AvailableMinutes += math.Max(0, tool.ResourceMinutes-used)
	}
	shortfall := (requiredMinutes - plan.AvailableMinutes) / tool.ResourceMinutes
	// Ignore floating-point noise at whole-tool boundaries.
	plan.PurchaseQuantity = math.Max(0, math.Ceil(shortfall-1e-9))
	plan.PurchaseCost = plan.PurchaseQuantity * tool.UnitPrice
	return plan
}

func (service *Service) updateCompanyEconomics(ctx context.Context, item model.CompanyEconomics) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.SaveCompanyEconomics(ctx, item)
}

func (service *Service) savePersonnelRate(ctx context.Context, item model.PersonnelRate) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.SavePersonnelRate(ctx, item)
}

func (service *Service) saveCompanyOverhead(ctx context.Context, item model.CompanyOverhead) error {
	repository, err := service.economicsRepository()
	if err != nil {
		return err
	}
	return repository.SaveCompanyOverhead(ctx, item)
}

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"smetchic/model"
)

func (service *Service) CreateEstimate(ctx context.Context, companyID string, input EstimateInput) (EstimateRecord, error) {
	input.PartRevisionID = strings.TrimSpace(input.PartRevisionID)
	input.Name = strings.TrimSpace(input.Name)
	normalizeEstimateSources(&input)
	if input.Name == "" {
		return EstimateRecord{}, Invalid("Название сметы обязательно.", nil)
	}
	detail, err := service.repository.GetRevisionEstimate(ctx, companyID, input.PartRevisionID)
	if err != nil {
		return EstimateRecord{}, err
	}
	if input.ProjectID == "" {
		input.ProjectID = detail.Project.ID
	}
	if input.ProjectID != detail.Project.ID {
		return EstimateRecord{}, Invalid("Деталь принадлежит другому проекту.", nil)
	}
	if err := validateEstimateInput(input); err != nil {
		return EstimateRecord{}, Invalid("Проверьте параметры сметы.", err)
	}
	if err := service.validateEstimateOrder(ctx, companyID, input.ProjectID, input.OrderID); err != nil {
		return EstimateRecord{}, err
	}
	repository, err := service.estimateDocumentRepository()
	if err != nil {
		return EstimateRecord{}, err
	}
	workingInput, err := json.Marshal(input)
	if err != nil {
		return EstimateRecord{}, fmt.Errorf("serialize working estimate input: %w", err)
	}
	id, err := newID("estimate")
	if err != nil {
		return EstimateRecord{}, err
	}
	estimate := EstimateRecord{
		ID: id, CompanyID: companyID, PartID: detail.Part.ID, ProjectID: input.ProjectID,
		OrderID: input.OrderID, PartRevisionID: input.PartRevisionID, Name: input.Name,
		Quantity: input.Quantity, Status: EstimateStatusDraft, WorkingInput: workingInput,
	}
	return repository.CreateEstimate(ctx, estimate)
}

func (service *Service) UpdateEstimateInput(ctx context.Context, companyID, estimateID string, input EstimateInput) (EstimateRecord, error) {
	current, err := service.repository.GetEstimate(ctx, companyID, estimateID)
	if err != nil {
		return EstimateRecord{}, err
	}
	input.PartRevisionID = current.PartRevisionID
	input.Name = strings.TrimSpace(input.Name)
	normalizeEstimateSources(&input)
	if input.Name == "" {
		return EstimateRecord{}, Invalid("Название сметы обязательно.", nil)
	}
	detail, err := service.repository.GetRevisionEstimate(ctx, companyID, current.PartRevisionID)
	if err != nil {
		return EstimateRecord{}, err
	}
	if input.ProjectID == "" {
		input.ProjectID = detail.Project.ID
	}
	if input.ProjectID != current.ProjectID || input.ProjectID != detail.Project.ID {
		return EstimateRecord{}, Invalid("Смету нельзя перенести в другой проект.", nil)
	}
	if err := validateEstimateInput(input); err != nil {
		return EstimateRecord{}, Invalid("Проверьте параметры сметы.", err)
	}
	if err := service.validateEstimateOrder(ctx, companyID, input.ProjectID, input.OrderID); err != nil {
		return EstimateRecord{}, err
	}
	workingInput, err := json.Marshal(input)
	if err != nil {
		return EstimateRecord{}, fmt.Errorf("serialize working estimate input: %w", err)
	}
	current.Name, current.OrderID = input.Name, input.OrderID
	current.Quantity, current.WorkingInput = input.Quantity, workingInput
	current.Status = EstimateStatusDraft
	repository, err := service.estimateDocumentRepository()
	if err != nil {
		return EstimateRecord{}, err
	}
	return repository.UpdateEstimateInput(ctx, companyID, current)
}

func (service *Service) ListCompanyEstimates(ctx context.Context, companyID string) ([]EstimateRecord, error) {
	repository, err := service.estimateDocumentRepository()
	if err != nil {
		return nil, err
	}
	return repository.ListCompanyEstimates(ctx, companyID)
}

func (service *Service) GetEstimateDocument(ctx context.Context, companyID, estimateID string) (EstimateDocument, error) {
	repository, err := service.estimateDocumentRepository()
	if err != nil {
		return EstimateDocument{}, err
	}
	return repository.GetEstimateDocument(ctx, companyID, estimateID)
}

func (service *Service) CalculateEstimate(ctx context.Context, companyID, estimateID string, input EstimateInput) (EstimateRecord, EstimateVersionRecord, error) {
	current, err := service.repository.GetEstimate(ctx, companyID, estimateID)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	normalizeEstimateSources(&input)
	if input.Name == "" {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Название сметы обязательно.", nil)
	}
	input.PartRevisionID = current.PartRevisionID
	input.ProjectID = current.ProjectID
	if err := validateEstimateInput(input); err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, Invalid("Проверьте параметры сметы.", err)
	}
	if err := service.validateEstimateOrder(ctx, companyID, current.ProjectID, input.OrderID); err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	workingInput, err := json.Marshal(input)
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, fmt.Errorf("serialize working estimate input: %w", err)
	}
	savedEstimate, version, err := service.CalculateWithProductionEconomics(
		ctx, companyID, current.PartRevisionID, input.Name, input.EquipmentID, input.MarkupPercent,
		ProductionEstimateOptions{
			EstimateID: estimateID, DocumentInput: input, Quantity: input.Quantity,
			SetupTimeMinutes: input.SetupTimeMinutes, ProgrammingHours: input.ProgrammingHours,
			MaterialID: input.MaterialID, GlobalMaterialID: input.GlobalMaterialID,
			StockBlankID: input.StockBlankID, ConsumableID: input.ConsumableID,
			ToolUsageMinutes: input.ToolUsageMinutes, MachineTimeOverride: input.MachineTime,
			EstimateWorkingInput: workingInput, OrderID: input.OrderID,
		})
	if err != nil {
		return EstimateRecord{}, EstimateVersionRecord{}, err
	}
	if savedEstimate.Number == "" {
		savedEstimate.Number = current.Number
	}
	return savedEstimate, version, nil
}

func (service *Service) estimateDocumentRepository() (EstimateDocumentRepository, error) {
	repository, ok := service.repository.(EstimateDocumentRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support working estimate documents")
	}
	return repository, nil
}

func (service *Service) validateEstimateOrder(ctx context.Context, companyID, projectID, orderID string) error {
	if strings.TrimSpace(orderID) == "" {
		return nil
	}
	order, err := service.repository.GetOrder(ctx, companyID, orderID)
	if err != nil {
		return err
	}
	if order.ProjectID != "" && order.ProjectID != projectID {
		return Invalid("Заказ связан с другим проектом.", nil)
	}
	return nil
}

func validateEstimateInput(input EstimateInput) error {
	if err := input.Modules.Validate(); err != nil {
		return err
	}
	var fixed model.ProjectLabor
	if input.ProjectLabor != nil {
		fixed = *input.ProjectLabor
		if err := fixed.Validate(); err != nil {
			return err
		}
	}
	if input.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	if strings.TrimSpace(input.EquipmentID) == "" {
		return fmt.Errorf("equipment is required")
	}
	for name, value := range map[string]float64{
		"material weight": input.MaterialWeightKg,
		"tool usage":      input.ToolUsageMinutes, "setup time": input.SetupTimeMinutes,
		"programming time": input.ProgrammingHours, "markup": input.MarkupPercent,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("%s must be finite and non-negative", name)
		}
	}
	switch input.MarkupSource {
	case "", EstimateSourceCompanyData, EstimateSourceCatalog, EstimateSourceAutomatic:
	case EstimateSourceUserOverride, EstimateSourceAIOverride:
		if strings.TrimSpace(input.MarkupReason) == "" {
			return fmt.Errorf("manual markup override requires a reason")
		}
	default:
		return fmt.Errorf("unsupported markup source %q", input.MarkupSource)
	}
	for _, field := range []struct {
		name   string
		value  float64
		source EstimateValueSource
		reason string
		fixed  *float64
	}{
		{"setup time", input.SetupTimeMinutes, input.SetupSource, input.SetupReason, fixed.Setup},
		{"programming time", input.ProgrammingHours, input.ProgrammingSource, input.ProgrammingReason, fixed.Programming},
		{"tool usage", input.ToolUsageMinutes, input.ToolUsageSource, input.ToolUsageReason, nil},
	} {
		switch field.source {
		case "", EstimateSourceAutomatic, EstimateSourceCatalog, EstimateSourceCompanyData,
			EstimateSourceUserOverride, EstimateSourceAIOverride:
		default:
			return fmt.Errorf("unsupported %s source %q", field.name, field.source)
		}
		if field.fixed == nil && field.value > 0 && field.source == EstimateSourceUserOverride &&
			strings.TrimSpace(field.reason) == "" {
			return fmt.Errorf("%s override requires a reason", field.name)
		}
	}
	if input.MachineTime != nil {
		if math.IsNaN(input.MachineTime.Value) || math.IsInf(input.MachineTime.Value, 0) || input.MachineTime.Value < 0 {
			return fmt.Errorf("machine time must be finite and non-negative")
		}
		if input.MachineTime.Source != EstimateSourceUserOverride && input.MachineTime.Source != EstimateSourceAIOverride {
			return fmt.Errorf("machine time override must have a manual or AI override source")
		}
	}
	if input.StockBlankID != "" && (input.MaterialID != "" || input.GlobalMaterialID != "") ||
		input.MaterialID != "" && input.GlobalMaterialID != "" {
		return fmt.Errorf("only one material cost source may be selected")
	}
	if input.ConsumableID != "" && input.GlobalToolID != "" {
		return fmt.Errorf("выберите инструмент только из одного источника")
	}
	return nil
}

func normalizeEstimateSources(input *EstimateInput) {
	input.clearInactiveModules()
	if input.QuantitySource == "" {
		input.QuantitySource = EstimateSourceUserOverride
	}
	if input.EquipmentSource == "" {
		input.EquipmentSource = EstimateSourceCompanyData
	}
	if input.MaterialID != "" && input.MaterialSource == "" {
		input.MaterialSource = EstimateSourceCompanyData
	}
	if input.GlobalMaterialID != "" && input.MaterialSource == "" {
		input.MaterialSource = EstimateSourceCatalog
	}
	if input.StockBlankID != "" && input.StockBlankSource == "" {
		input.StockBlankSource = EstimateSourceCompanyData
	}
	if input.ConsumableID != "" && input.ToolSource == "" {
		input.ToolSource = EstimateSourceCompanyData
	}
	if input.GlobalToolID != "" {
		input.ToolSource = EstimateSourceCatalog
	} else if input.ConsumableID != "" {
		input.ToolSource = EstimateSourceCompanyData
	}
	if input.ToolUsageMinutes > 0 && input.ToolUsageSource == "" {
		input.ToolUsageSource = EstimateSourceUserOverride
	}
	if input.SetupTimeMinutes > 0 && input.SetupSource == "" {
		input.SetupSource = EstimateSourceUserOverride
	}
	if input.ProgrammingHours > 0 && input.ProgrammingSource == "" {
		input.ProgrammingSource = EstimateSourceUserOverride
	}
	if input.MarkupSource == "" {
		input.MarkupSource = EstimateSourceCompanyData
	}
}

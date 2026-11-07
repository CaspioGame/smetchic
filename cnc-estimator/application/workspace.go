package application

import (
	"context"
	"encoding/json"
	"strings"

	"smetchic/model"
)

func (service *Service) UpdateCompany(ctx context.Context, company model.Company, settings model.CompanySettings) error {
	company.Name = strings.TrimSpace(company.Name)
	if err := company.Validate(); err != nil {
		return Invalid("Укажите корректные данные компании.", err)
	}
	if err := settings.Validate(); err != nil {
		return Invalid("Проверьте настройки компании.", err)
	}
	if company.ID != settings.CompanyID {
		return Invalid("Настройки не принадлежат этой компании.", nil)
	}
	return service.repository.UpdateCompany(ctx, company, settings)
}

func (service *Service) ListCompanyUsers(ctx context.Context, companyID string) ([]CompanyUserRecord, error) {
	return service.repository.ListCompanyUsers(ctx, companyID)
}

func (service *Service) ListEquipmentModels(ctx context.Context) ([]model.EquipmentModel, error) {
	return service.repository.ListEquipmentModels(ctx)
}

func (service *Service) GetEquipment(ctx context.Context, companyID, equipmentID string) (model.Equipment, error) {
	return service.config.GetEquipment(ctx, companyID, equipmentID)
}

func (service *Service) SaveEquipment(ctx context.Context, companyID string, item model.Equipment) (model.Equipment, error) {
	if item.ID == "" {
		id, err := newID("equipment")
		if err != nil {
			return model.Equipment{}, err
		}
		item.ID = id
	}
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return model.Equipment{}, Invalid("Проверьте данные оборудования.", err)
	}
	if err := service.repository.SaveEquipment(ctx, item); err != nil {
		return model.Equipment{}, err
	}
	return service.config.GetEquipment(ctx, companyID, item.ID)
}

func (service *Service) DeleteEquipment(ctx context.Context, companyID, equipmentID string) error {
	return service.repository.DeleteEquipment(ctx, companyID, equipmentID)
}

func (service *Service) ListMaterials(ctx context.Context, companyID string) ([]model.Material, error) {
	return service.repository.ListMaterials(ctx, companyID)
}

func (service *Service) SaveMaterial(ctx context.Context, companyID string, item model.Material) (model.Material, error) {
	if item.ID == "" {
		id, err := newID("material")
		if err != nil {
			return model.Material{}, err
		}
		item.ID = id
	}
	item.CompanyID = companyID
	if err := item.Validate(); err != nil {
		return model.Material{}, Invalid("Проверьте данные материала.", err)
	}
	if err := service.repository.SaveMaterial(ctx, item); err != nil {
		return model.Material{}, err
	}
	items, err := service.repository.ListMaterials(ctx, companyID)
	if err != nil {
		return model.Material{}, err
	}
	for _, saved := range items {
		if saved.ID == item.ID {
			return saved, nil
		}
	}
	return model.Material{}, ErrNotFound
}

func (service *Service) DeleteMaterial(ctx context.Context, companyID, materialID string) error {
	return service.repository.DeleteMaterial(ctx, companyID, materialID)
}

func (service *Service) ListGlobalMaterials(ctx context.Context) ([]model.GlobalMaterial, error) {
	return service.repository.ListGlobalMaterials(ctx)
}

func (service *Service) ListConsumables(ctx context.Context, companyID string) ([]model.Consumable, error) {
	return service.repository.ListConsumables(ctx, companyID)
}

func (service *Service) SaveConsumable(ctx context.Context, companyID string, item model.Consumable) (model.Consumable, error) {
	if item.ID == "" {
		id, err := newID("consumable")
		if err != nil {
			return model.Consumable{}, err
		}
		item.ID = id
	}
	item.CompanyID = companyID
	if len(item.Attributes) == 0 {
		item.Attributes = json.RawMessage(`{}`)
	}
	if err := item.Validate(); err != nil {
		return model.Consumable{}, Invalid("Проверьте данные расходника.", err)
	}
	if err := service.repository.SaveConsumable(ctx, item); err != nil {
		return model.Consumable{}, err
	}
	items, err := service.repository.ListConsumables(ctx, companyID)
	if err != nil {
		return model.Consumable{}, err
	}
	for _, saved := range items {
		if saved.ID == item.ID {
			return saved, nil
		}
	}
	return model.Consumable{}, ErrNotFound
}

func (service *Service) DeleteConsumable(ctx context.Context, companyID, consumableID string) error {
	return service.repository.DeleteConsumable(ctx, companyID, consumableID)
}

func (service *Service) ListOrders(ctx context.Context, companyID string) ([]OrderRecord, error) {
	return service.repository.ListOrders(ctx, companyID)
}

func (service *Service) GetOrder(ctx context.Context, companyID, orderID string) (OrderRecord, error) {
	return service.repository.GetOrder(ctx, companyID, orderID)
}

func (service *Service) CreateOrder(
	ctx context.Context,
	companyID, userID, projectID, name, description string,
) (OrderRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return OrderRecord{}, Invalid("Название заказа обязательно.", nil)
	}
	id, err := newID("order")
	if err != nil {
		return OrderRecord{}, err
	}
	order := OrderRecord{
		ID: id, CompanyID: companyID, CreatedByID: userID,
		ProjectID: strings.TrimSpace(projectID), Name: name,
		Description: strings.TrimSpace(description), Status: OrderStatusDraft,
		Attributes: json.RawMessage(`{}`),
	}
	if err := service.repository.CreateOrder(ctx, order); err != nil {
		return OrderRecord{}, err
	}
	return service.repository.GetOrder(ctx, companyID, id)
}

func (service *Service) UpdateOrder(ctx context.Context, order OrderRecord) (OrderRecord, error) {
	order.Name = strings.TrimSpace(order.Name)
	order.Description = strings.TrimSpace(order.Description)
	order.ProjectID = strings.TrimSpace(order.ProjectID)
	if order.Name == "" {
		return OrderRecord{}, Invalid("Название заказа обязательно.", nil)
	}
	switch order.Status {
	case OrderStatusDraft, OrderStatusInProgress, OrderStatusCompleted, OrderStatusCancelled:
	default:
		return OrderRecord{}, Invalid("Выберите допустимый статус заказа.", nil)
	}
	if err := service.repository.UpdateOrder(ctx, order); err != nil {
		return OrderRecord{}, err
	}
	return service.repository.GetOrder(ctx, order.CompanyID, order.ID)
}

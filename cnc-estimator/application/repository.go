package application

import (
	"context"
	"errors"

	"smetchic/model"
)

var ErrNotFound = errors.New("record not found")

type Repository interface {
	EnsureCompany(context.Context, model.Company, model.CompanySettings) error
	EnsureEquipmentModel(context.Context, model.EquipmentModel) error
	EnsureEquipment(context.Context, model.Equipment) error
	GetCompany(context.Context, string) (model.Company, error)
	GetCompanySettings(context.Context, string) (model.CompanySettings, error)
	UpdateCompany(context.Context, model.Company, model.CompanySettings) error
	ListCompanyUsers(context.Context, string) ([]CompanyUserRecord, error)
	ListEquipmentModels(context.Context) ([]model.EquipmentModel, error)
	SaveEquipment(context.Context, model.Equipment) error
	DeleteEquipment(context.Context, string, string) error
	ListMaterials(context.Context, string) ([]model.Material, error)
	SaveMaterial(context.Context, model.Material) error
	DeleteMaterial(context.Context, string, string) error
	ListGlobalMaterials(context.Context) ([]model.GlobalMaterial, error)
	ListConsumables(context.Context, string) ([]model.Consumable, error)
	SaveConsumable(context.Context, model.Consumable) error
	DeleteConsumable(context.Context, string, string) error
	ListOrders(context.Context, string) ([]OrderRecord, error)
	CreateOrder(context.Context, OrderRecord) error
	UpdateOrder(context.Context, OrderRecord) error
	GetOrder(context.Context, string, string) (OrderRecord, error)
	CreateProject(context.Context, ProjectRecord) error
	UpdateProject(context.Context, ProjectRecord) (ProjectRecord, error)
	GetProject(context.Context, string, string) (ProjectRecord, error)
	ListProjects(context.Context, string) ([]ProjectRecord, error)

	GetPart(context.Context, string, string) (PartRecord, error)
	ListParts(context.Context, string, string) ([]PartRecord, error)
	CreatePartWithRevision(context.Context, PartRecord, PartRevisionRecord) (PartRecord, PartRevisionRecord, error)
	GetRevisionEstimate(context.Context, string, string) (RevisionEstimate, error)
	ListPartRevisions(context.Context, string, string) ([]PartRevisionRecord, error)

	ListEstimatesForRevision(context.Context, string, string) ([]EstimateRecord, error)
	CreateEstimateVersion(context.Context, EstimateRecord, EstimateVersionRecord) (EstimateRecord, EstimateVersionRecord, error)
	GetEstimate(context.Context, string, string) (EstimateRecord, error)
	GetEstimateVersion(context.Context, string, string, string) (EstimateVersionDetail, error)
	ListEstimateVersions(context.Context, string, string) ([]EstimateVersionRecord, error)
}

type ConfigProvider interface {
	GetCompany(context.Context, string) (model.Company, error)
	GetCompanySettings(context.Context, string) (model.CompanySettings, error)
	GetEquipment(context.Context, string, string) (model.Equipment, error)
	ListEquipment(context.Context, string) ([]model.Equipment, error)
}

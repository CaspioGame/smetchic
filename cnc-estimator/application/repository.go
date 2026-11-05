package application

import (
	"context"
	"errors"

	"smetchic/model"
)

var ErrNotFound = errors.New("record not found")

type Repository interface {
	EnsureCompany(context.Context, model.Company, model.CompanySettings) error
	CreateProject(context.Context, ProjectRecord) error
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
	GetCompanySettings(context.Context, string) (model.CompanySettings, error)
	GetEquipment(context.Context, string, string) (model.Equipment, error)
}

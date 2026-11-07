package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"smetchic/model"
	"smetchic/parser"
)

const ParserVersion = "part-parser-v1"

type Service struct {
	repository Repository
	config     ConfigProvider
}

func NewService(repository Repository, config ConfigProvider) *Service {
	return &Service{repository: repository, config: config}
}

func (service *Service) EnsureCompany(ctx context.Context, company model.Company, settings model.CompanySettings) error {
	if err := company.Validate(); err != nil {
		return err
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	if company.ID != settings.CompanyID {
		return fmt.Errorf("company and settings IDs do not match")
	}
	return service.repository.EnsureCompany(ctx, company, settings)
}

func (service *Service) EnsureEquipmentModel(ctx context.Context, equipmentModel model.EquipmentModel) error {
	if err := equipmentModel.Validate(); err != nil {
		return err
	}
	return service.repository.EnsureEquipmentModel(ctx, equipmentModel)
}

func (service *Service) EnsureEquipment(ctx context.Context, equipment model.Equipment) error {
	if err := equipment.Validate(); err != nil {
		return err
	}
	return service.repository.EnsureEquipment(ctx, equipment)
}

func (service *Service) InitializeDemoCompany(ctx context.Context) (model.Company, error) {
	demo := DefaultDemoData()
	if err := service.EnsureCompany(ctx, demo.Company, demo.Settings); err != nil {
		return model.Company{}, err
	}
	if err := service.EnsureEquipmentModel(ctx, demo.EquipmentModel); err != nil {
		return model.Company{}, err
	}
	if err := service.EnsureEquipment(ctx, demo.Equipment); err != nil {
		return model.Company{}, err
	}
	return service.GetCompany(ctx, demo.Company.ID)
}

func (service *Service) GetCompany(ctx context.Context, companyID string) (model.Company, error) {
	return service.config.GetCompany(ctx, companyID)
}

func (service *Service) GetCompanySettings(ctx context.Context, companyID string) (model.CompanySettings, error) {
	return service.config.GetCompanySettings(ctx, companyID)
}

func (service *Service) ListEquipment(ctx context.Context, companyID string) ([]model.Equipment, error) {
	return service.config.ListEquipment(ctx, companyID)
}

func (service *Service) ListProjects(ctx context.Context, companyID string) ([]ProjectRecord, error) {
	return service.repository.ListProjects(ctx, companyID)
}

func (service *Service) CreateProject(ctx context.Context, companyID, name, description string) (ProjectRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectRecord{}, Invalid("Название проекта обязательно.", nil)
	}
	id, err := newID("project")
	if err != nil {
		return ProjectRecord{}, err
	}
	project := ProjectRecord{
		ID: id, CompanyID: companyID, Name: name,
		Description: strings.TrimSpace(description),
	}
	if err := service.repository.CreateProject(ctx, project); err != nil {
		return ProjectRecord{}, err
	}
	return service.repository.GetProject(ctx, companyID, id)
}

func (service *Service) GetProject(ctx context.Context, companyID, projectID string) (ProjectRecord, error) {
	return service.repository.GetProject(ctx, companyID, projectID)
}

func (service *Service) UpdateProject(ctx context.Context, companyID, projectID, name, description string) (ProjectRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectRecord{}, Invalid("Название проекта обязательно.", nil)
	}
	return service.repository.UpdateProject(ctx, ProjectRecord{
		ID: projectID, CompanyID: companyID, Name: name,
		Description: strings.TrimSpace(description),
	})
}

func (service *Service) ListParts(ctx context.Context, companyID, projectID string) ([]PartRecord, error) {
	if _, err := service.repository.GetProject(ctx, companyID, projectID); err != nil {
		return nil, err
	}
	return service.repository.ListParts(ctx, companyID, projectID)
}

func (service *Service) GetPart(ctx context.Context, companyID, partID string) (PartRecord, error) {
	return service.repository.GetPart(ctx, companyID, partID)
}

func (service *Service) UploadRevision(ctx context.Context, companyID, projectID string, upload RevisionUpload) (PartRecord, PartRevisionRecord, error) {
	project, err := service.repository.GetProject(ctx, companyID, projectID)
	if err != nil {
		return PartRecord{}, PartRevisionRecord{}, err
	}
	filename := filepath.Base(strings.TrimSpace(upload.SourceFilename))
	if filename == "" || filename == "." || !strings.EqualFold(filepath.Ext(filename), ".part") {
		return PartRecord{}, PartRevisionRecord{}, Invalid("Требуется корректный файл с расширением .part.", nil)
	}
	if len(upload.SourceContent) == 0 {
		return PartRecord{}, PartRevisionRecord{}, Invalid("Файл .part пуст.", nil)
	}
	partName := strings.TrimSpace(upload.PartName)
	if partName == "" {
		partName = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	if partName == "" {
		return PartRecord{}, PartRevisionRecord{}, Invalid("Название детали обязательно.", nil)
	}
	parsed, err := parser.Parse(strings.NewReader(string(upload.SourceContent)), filename)
	if err != nil {
		return PartRecord{}, PartRevisionRecord{}, Invalid("Не удалось разобрать файл .part", err)
	}
	partID, err := newID("part")
	if err != nil {
		return PartRecord{}, PartRevisionRecord{}, err
	}
	revisionID, err := newID("revision")
	if err != nil {
		return PartRecord{}, PartRevisionRecord{}, err
	}
	hash := sha256.Sum256(upload.SourceContent)
	part := PartRecord{
		ID: partID, CompanyID: companyID, ProjectID: project.ID,
		Name: partName, Description: strings.TrimSpace(upload.PartDescription),
	}
	revision := PartRevisionRecord{
		ID: revisionID, CompanyID: companyID, PartID: partID,
		SourceFormat: "part", SourceFilename: filename,
		SourceContent: string(upload.SourceContent),
		SourceHash:    hex.EncodeToString(hash[:]), ParserVersion: ParserVersion,
		ParsedPart: parsed,
	}
	return service.repository.CreatePartWithRevision(ctx, part, revision)
}

func (service *Service) GetRevisionEstimate(ctx context.Context, companyID, revisionID string) (RevisionEstimate, error) {
	return service.repository.GetRevisionEstimate(ctx, companyID, revisionID)
}

func (service *Service) ListPartRevisions(ctx context.Context, companyID, partID string) ([]PartRevisionRecord, error) {
	return service.repository.ListPartRevisions(ctx, companyID, partID)
}

func (service *Service) ListEstimatesForRevision(ctx context.Context, companyID, revisionID string) ([]EstimateRecord, error) {
	return service.repository.ListEstimatesForRevision(ctx, companyID, revisionID)
}

func (service *Service) Calculate(ctx context.Context, companyID, revisionID, estimateName, equipmentID string, markup float64) (EstimateRecord, EstimateVersionRecord, error) {
	return service.CalculateWithProductionEconomics(ctx, companyID, revisionID, estimateName, equipmentID, markup,
		ProductionEstimateOptions{Quantity: 1})
}

func (service *Service) GetEstimate(ctx context.Context, companyID, estimateID string) (EstimateRecord, error) {
	return service.repository.GetEstimate(ctx, companyID, estimateID)
}

func (service *Service) GetEstimateVersion(ctx context.Context, companyID, estimateID, versionID string) (EstimateVersionDetail, error) {
	return service.repository.GetEstimateVersion(ctx, companyID, estimateID, versionID)
}

func (service *Service) ListEstimateVersions(ctx context.Context, companyID, estimateID string) ([]EstimateVersionRecord, error) {
	return service.repository.ListEstimateVersions(ctx, companyID, estimateID)
}

func newID(prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", fmt.Errorf("generate ID: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(value), nil
}

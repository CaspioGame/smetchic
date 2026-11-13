package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"smetchic/model"
)

type ChatDialog struct {
	ID, CompanyID, UserID, Title string
	State                        json.RawMessage
	Revision                     int
	Processing                   bool
	UpdatedAt                    time.Time
}

type ChatRepository interface {
	CreateChat(context.Context, ChatDialog) (ChatDialog, error)
	ListChats(context.Context, string, string) ([]ChatDialog, error)
	GetChat(context.Context, string, string, string) (ChatDialog, error)
	ClaimChat(context.Context, string, string, string, int) error
	SaveChat(context.Context, ChatDialog) error
	CreateChatEstimate(context.Context, EstimateRecord, PartRecord, PartRevisionRecord) (EstimateRecord, error)
}

func (s *Service) ChatRepository() (ChatRepository, error) {
	repo, ok := s.repository.(ChatRepository)
	if !ok {
		return nil, fmt.Errorf("хранилище чата недоступно")
	}
	return repo, nil
}

func (s *Service) NewChat(ctx context.Context, companyID, userID string) (ChatDialog, error) {
	repo, err := s.ChatRepository()
	if err != nil {
		return ChatDialog{}, err
	}
	id, err := newID("chat")
	if err != nil {
		return ChatDialog{}, err
	}
	return repo.CreateChat(ctx, ChatDialog{ID: id, CompanyID: companyID, UserID: userID, Title: "Новый диалог", State: json.RawMessage("{}")})
}

// A manual revision keeps the existing estimate lifecycle available before a file is supplied.
func (s *Service) CreateManualEstimate(ctx context.Context, companyID, projectID, name, equipmentID string, quantity int) (EstimateRecord, error) {
	settings, err := s.GetCompanySettings(ctx, companyID)
	if err != nil {
		return EstimateRecord{}, err
	}
	return s.CreateManualEstimateInput(ctx, companyID, projectID, EstimateInput{Name: name, EquipmentID: equipmentID, Quantity: quantity,
		Modules: EstimateModules{}, MarkupPercent: settings.DefaultMarkupPercent, MarkupSource: EstimateSourceCompanyData})
}

func (s *Service) CreateManualEstimateInput(ctx context.Context, companyID, projectID string, input EstimateInput) (EstimateRecord, error) {
	project, err := s.GetProject(ctx, companyID, projectID)
	if err != nil {
		return EstimateRecord{}, err
	}
	equipment, err := s.config.GetEquipment(ctx, companyID, input.EquipmentID)
	if err != nil {
		return EstimateRecord{}, err
	}
	if equipment.Status != model.EquipmentStatusActive {
		return EstimateRecord{}, Invalid("Выберите активный станок компании.", nil)
	}
	input.Name, input.ProjectID = strings.TrimSpace(input.Name), project.ID
	normalizeEstimateSources(&input)
	if input.Name == "" {
		return EstimateRecord{}, Invalid("Укажите название сметы.", nil)
	}
	if err := validateEstimateInput(input); err != nil {
		return EstimateRecord{}, Invalid("Проверьте параметры сметы.", err)
	}
	if err := s.validateEstimateOrder(ctx, companyID, project.ID, input.OrderID); err != nil {
		return EstimateRecord{}, err
	}
	repo, err := s.ChatRepository()
	if err != nil {
		return EstimateRecord{}, err
	}
	estimateID, err := newID("estimate")
	if err != nil {
		return EstimateRecord{}, err
	}
	partID, err := newID("part")
	if err != nil {
		return EstimateRecord{}, err
	}
	revisionID, err := newID("revision")
	if err != nil {
		return EstimateRecord{}, err
	}
	input.PartRevisionID = revisionID
	data, err := json.Marshal(input)
	if err != nil {
		return EstimateRecord{}, err
	}
	return repo.CreateChatEstimate(ctx,
		EstimateRecord{ID: estimateID, CompanyID: companyID, ProjectID: project.ID, PartID: partID, PartRevisionID: revisionID,
			Name: input.Name, Quantity: input.Quantity, OrderID: input.OrderID, WorkingInput: data},
		PartRecord{ID: partID, CompanyID: companyID, ProjectID: project.ID, Name: input.Name},
		PartRevisionRecord{ID: revisionID, CompanyID: companyID, PartID: partID, SourceFormat: "manual", ParserVersion: attachmentVersion})
}

package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"smetchic/application"
	"smetchic/model"
)

type dialogState struct {
	ProjectID  string
	EstimateID string
	Turns      []estimateChatTurn
}

type chatItem struct{ ID, Name string }
type chatCatalog struct {
	CurrentProjectID                                                                                    string `json:"current_project_id"`
	CurrentEstimateID                                                                                   string `json:"current_estimate_id"`
	Projects, Estimates, Equipment, Materials, GlobalMaterials, StockBlanks, Tools, GlobalTools, Orders []chatItem
}

var chatTextFields = []string{"name", "order_id", "equipment_id", "material_id", "global_material_id", "stock_blank_id", "consumable_id", "global_tool_id", "machine_time_reason", "markup_reason", "setup_reason", "programming_reason", "tool_usage_reason"}

func chatTextSchema() map[string]any {
	properties := map[string]any{}
	for _, key := range chatTextFields {
		properties[key] = map[string]string{"type": "string"}
	}
	return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
}

func (s *Server) dialogContext(ctx context.Context, companyID string, state dialogState) (chatCatalog, map[string]string, error) {
	catalog := chatCatalog{CurrentProjectID: state.ProjectID, CurrentEstimateID: state.EstimateID}
	values := createEstimateFormValues("0")
	settings, err := s.lifecycle.GetCompanySettings(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	values["markup_percent"] = formatNumber(settings.DefaultMarkupPercent)
	projects, err := s.lifecycle.ListProjects(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range projects {
		catalog.Projects = append(catalog.Projects, chatItem{item.ID, item.Name})
	}
	if state.ProjectID != "" {
		if _, err := s.lifecycle.GetProject(ctx, companyID, state.ProjectID); err != nil {
			return catalog, nil, err
		}
	}
	estimates, err := s.lifecycle.ListCompanyEstimates(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range estimates {
		catalog.Estimates = append(catalog.Estimates, chatItem{item.ID, item.Name + " (проект " + item.ProjectName + ")"})
	}
	if state.EstimateID != "" {
		doc, err := s.lifecycle.GetEstimateDocument(ctx, companyID, state.EstimateID)
		if err != nil {
			return catalog, nil, err
		}
		input, err := decodeWorkingEstimateInput(doc.Estimate.WorkingInput)
		if err != nil {
			return catalog, nil, err
		}
		if input.Name == "" {
			for _, version := range doc.Versions {
				if version.ID == doc.Estimate.CurrentVersionID {
					snapshot, err := decodeEstimateVersionSnapshot(version.InputSnapshot)
					if err != nil {
						return catalog, nil, err
					}
					input = snapshot.WorkingInput
				}
			}
			input.Name = doc.Estimate.Name
			input.ProjectID = doc.Estimate.ProjectID
			input.PartRevisionID = doc.Estimate.PartRevisionID
			input.Quantity = doc.Estimate.Quantity
		}
		values = estimateInputFormValues(input, 0, 0, false)
	}
	equipment, err := s.lifecycle.ListEquipment(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range equipment {
		if item.Status == model.EquipmentStatusActive {
			catalog.Equipment = append(catalog.Equipment, chatItem{item.ID, item.Name})
		}
	}
	materials, err := s.lifecycle.ListMaterials(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range materials {
		catalog.Materials = append(catalog.Materials, chatItem{item.ID, item.Name})
	}
	global, err := s.lifecycle.ListGlobalMaterials(ctx)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range global {
		catalog.GlobalMaterials = append(catalog.GlobalMaterials, chatItem{item.ID, item.Name + " " + item.Grade})
	}
	blanks, err := s.lifecycle.ListStockBlanks(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range blanks {
		catalog.StockBlanks = append(catalog.StockBlanks, chatItem{item.ID, item.Name})
	}
	tools, err := s.lifecycle.ListConsumables(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range tools {
		catalog.Tools = append(catalog.Tools, chatItem{item.ID, item.Name})
	}
	globalTools, err := s.lifecycle.ListGlobalTools(ctx)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range globalTools {
		catalog.GlobalTools = append(catalog.GlobalTools, chatItem{item.ID, item.Name})
	}
	orders, err := s.lifecycle.ListOrders(ctx, companyID)
	if err != nil {
		return catalog, nil, err
	}
	for _, item := range orders {
		catalog.Orders = append(catalog.Orders, chatItem{item.ID, item.Name})
	}
	return catalog, values, nil
}

func (s *Server) renderEstimateChat(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityFromRequest(r)
	if !ok {
		http.Error(w, "Для чата войдите в аккаунт.", http.StatusUnauthorized)
		return
	}
	repo, err := s.lifecycle.ChatRepository()
	if err != nil {
		s.writeInternalError(w, "load chat repository", err)
		return
	}
	companyID, userID := identity.Company.ID, identity.User.ID
	s.setCSRFCookie(w, r)
	page := s.lifecycleBase(r, "Чат", "chat", "chat")
	id := r.URL.Query().Get("dialog_id")
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if !s.parseMutationForm(w, r) {
			return
		}
		id = r.PostFormValue("dialog_id")
	}
	var chat application.ChatDialog
	var state dialogState
	if id != "" {
		chat, err = repo.GetChat(r.Context(), companyID, userID, id)
		if err != nil {
			s.lifecycleError(w, r, "get chat", err)
			return
		}
		if err = json.Unmarshal(chat.State, &state); err != nil {
			s.writeInternalError(w, "decode chat", err)
			return
		}
	}
	if r.Method == http.MethodPost {
		message := strings.TrimSpace(r.PostFormValue("assistant_prompt"))
		confirm := r.PostFormValue("confirm")
		if confirm == "" && (message == "" || len([]rune(message)) > 2000) {
			http.Error(w, "Введите сообщение от 1 до 2 000 символов.", http.StatusBadRequest)
			return
		}
		if id == "" {
			chat, err = s.lifecycle.NewChat(r.Context(), companyID, userID)
			if err != nil {
				s.writeInternalError(w, "create chat", err)
				return
			}
			id = chat.ID
			state.EstimateID = r.PostFormValue("estimate_id")
		} else {
			revision, parseErr := strconv.Atoi(r.PostFormValue("dialog_revision"))
			if parseErr != nil || revision != chat.Revision {
				http.Error(w, "Диалог обновился. Обновите страницу.", http.StatusConflict)
				return
			}
		}
		if err = repo.ClaimChat(r.Context(), companyID, userID, id, chat.Revision); err != nil {
			http.Error(w, "Диалог уже обрабатывает сообщение или изменился. Обновите страницу.", http.StatusConflict)
			return
		}
		chat.Revision++
		turn := estimateChatTurn{CompanyID: companyID}
		catalog, values, contextErr := s.dialogContext(r.Context(), companyID, state)
		if confirm != "" {
			turn.Message = "Подтвердить действие"
			if contextErr != nil {
				err = contextErr
			} else {
				err = application.Invalid("Предложение уже выполнено или недоступно.", nil)
				for i := len(state.Turns) - 1; i >= 0; i-- {
					previous := &state.Turns[i]
					if previous.ID != confirm || !previous.Pending {
						continue
					}
					previous.Pending = false
					if identity.Role == model.CompanyRoleOperator && previous.Proposal.Action != nil && previous.Proposal.Action.Operation == "create_project" {
						err = application.Invalid("Создавать проекты могут менеджер и администратор компании.", nil)
					} else if previous.BaseInput != "" && previous.BaseInput != chatInputHash(values) {
						err = application.Invalid("Параметры сметы изменились. Отправьте запрос заново.", nil)
					} else {
						// Consume the proposal durably before its side effect, so a
						// lost response cannot execute the same action twice.
						chat.State, err = json.Marshal(state)
						if err != nil {
							s.writeInternalError(w, "checkpoint chat", err)
							return
						}
						chat.Processing = true
						if err = repo.SaveChat(r.Context(), chat); err != nil {
							s.writeInternalError(w, "checkpoint chat", err)
							return
						}
						turn.Reply, err = s.executeChatAction(r.Context(), companyID, &state, previous.Proposal, catalog, values)
						chat.Processing = false
					}
					break
				}
			}
		} else {
			turn.Message = message
			if contextErr != nil {
				err = contextErr
			} else {
				modelName := strings.TrimSpace(os.Getenv("ESTIMATE_OLLAMA_MODEL"))
				if modelName == "" {
					modelName = "qwen3:4b"
				}
				if s.chatAssistant != nil {
					turn.Proposal, err = s.chatAssistant(r.Context(), estimateChatPrompt(state.Turns, message), values, catalog)
				} else {
					turn.Proposal, err = requestDialogAssistant(r.Context(), "http://127.0.0.1:11434/api/chat", modelName, estimateChatPrompt(state.Turns, message), values, catalog)
				}
				if err == nil {
					groundChatReferences(&turn.Proposal, message, catalog)
					turn.Reply = turn.Proposal.Reply
					if turn.Proposal.Action != nil && turn.Proposal.Action.Operation != "" && turn.Proposal.Action.Operation != "talk" {
						turn.BaseInput = chatInputHash(values)
						turn.Changes, err = validateChatAction(turn.Proposal, state, catalog, maps.Clone(values))
						if err == nil {
							turn.Reply = "Проверьте предложение. Действие будет выполнено после подтверждения."
							turn.Pending = true
							turn.ID, err = randomToken(12)
						}
					}
				}
			}
			if chat.Title == "Новый диалог" {
				title := []rune(message)
				chat.Title = string(title[:min(len(title), 60)])
			}
		}
		if err != nil {
			turn.Pending = false
			turn.Reply = chatPublicError(err)
		}
		turn.EstimateID = state.EstimateID
		// Only the latest proposal can be confirmed.
		for i := range state.Turns {
			state.Turns[i].Pending = false
		}
		state.Turns = append(state.Turns, turn)
		chat.State, err = json.Marshal(state)
		if err != nil {
			s.writeInternalError(w, "encode chat", err)
			return
		}
		// Finish the acquired turn even when the browser disconnects.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		if err = repo.SaveChat(ctx, chat); err != nil {
			s.writeInternalError(w, "save chat", err)
			return
		}
		http.Redirect(w, r, "/chat?dialog_id="+url.QueryEscape(id), http.StatusSeeOther)
		return
	}
	page.Dialogs, err = repo.ListChats(r.Context(), companyID, userID)
	if err != nil {
		s.writeInternalError(w, "list chats", err)
		return
	}
	page.DialogID, page.DialogRevision = chat.ID, chat.Revision
	page.ChatTurns = state.Turns
	if state.ProjectID != "" {
		page.Project, err = s.lifecycle.GetProject(r.Context(), companyID, state.ProjectID)
		if err != nil {
			s.lifecycleError(w, r, "get chat project", err)
			return
		}
	}
	page.ChatEstimateID = state.EstimateID
	if state.EstimateID != "" {
		document, err := s.lifecycle.GetEstimateDocument(r.Context(), companyID, state.EstimateID)
		if err != nil {
			s.lifecycleError(w, r, "get chat estimate", err)
			return
		}
		page.Estimate = document.Estimate
	}
	// Legacy estimate links open a fresh, independent conversation.
	if id == "" {
		page.ChatEstimateID = r.URL.Query().Get("estimate_id")
	}
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func chatPublicError(err error) string {
	var validation *application.ValidationError
	if errors.As(err, &validation) {
		return validation.Error()
	}
	if errors.Is(err, application.ErrNotFound) {
		return "Объект недоступен в вашей компании. Уточните название."
	}
	message := err.Error()
	for _, prefix := range []string{"Не удалось связаться с локальным", "Ollama ", "Модель ", "Сократите запрос"} {
		if strings.HasPrefix(message, prefix) {
			return message
		}
	}
	log.Printf("chat operation failed: %v", err)
	return "Не удалось выполнить действие. Повторите запрос или проверьте параметры."
}

func chatInputHash(values map[string]string) string {
	data, _ := json.Marshal(values)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

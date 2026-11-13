package web

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"smetchic/application"
)

func chatItemName(items []chatItem, id string) (string, error) {
	for _, item := range items {
		if item.ID == id {
			return item.Name, nil
		}
	}
	return "", application.ErrNotFound
}

func prepareChatInput(reply estimateAssistantReply, values map[string]string, catalog chatCatalog) (application.EstimateInput, []string, error) {
	reply.Changes = maps.Clone(reply.Changes)
	for key, number := range reply.Changes {
		old, oldErr := strconv.ParseFloat(values[key], 64)
		next, nextErr := number.Float64()
		if oldErr == nil && nextErr == nil && old == next {
			delete(reply.Changes, key)
		}
	}
	labels, err := assistantFormChanges(values, reply)
	if err != nil {
		return application.EstimateInput{}, nil, err
	}
	references := map[string][]chatItem{"equipment_id": catalog.Equipment, "order_id": catalog.Orders,
		"material_id": catalog.Materials, "global_material_id": catalog.GlobalMaterials, "stock_blank_id": catalog.StockBlanks,
		"consumable_id": catalog.Tools, "global_tool_id": catalog.GlobalTools}
	for key, value := range reply.TextChanges {
		if !slices.Contains(chatTextFields, key) || len([]rune(value)) > 500 {
			return application.EstimateInput{}, nil, application.Invalid("Проверьте текстовые поля предложения.", nil)
		}
		label := key
		if items, isReference := references[key]; isReference && value != "" {
			label, err = chatItemName(items, value)
			if err != nil {
				return application.EstimateInput{}, nil, err
			}
		}
		values[key] = value
		labels = append(labels, label+" → "+value)
	}
	if action := reply.Action; action != nil {
		if action.Name != "" && action.Operation == "create_estimate" {
			values["name"] = action.Name
		}
		if action.EquipmentID != "" {
			values["equipment_id"] = action.EquipmentID
		}
	}
	if id := values["equipment_id"]; id != "" {
		if _, err := chatItemName(catalog.Equipment, id); err != nil {
			return application.EstimateInput{}, nil, err
		}
	}
	// Choosing a catalog item activates its module and clears the competing source.
	for _, module := range application.EstimateModuleCatalog() {
		for key, value := range reply.TextChanges {
			if _, ref := references[key]; !ref || key == "equipment_id" || key == "order_id" || value == "" || !slices.Contains(module.Fields, key) {
				continue
			}
			for _, other := range application.EstimateModuleCatalog() {
				if module.Group != "" && module.ID != other.ID && module.Group == other.Group {
					delete(values, "module_"+other.ID)
					for _, field := range other.Fields {
						if field != "material_weight_kg" && field != "tool_usage_minutes" && field != "tool_usage_reason" {
							delete(values, field)
						}
					}
				}
			}
			values["module_"+module.ID] = "1"
		}
	}
	form := url.Values{"estimate_modules_present": {"1"}}
	for key, value := range values {
		form.Set(key, value)
	}
	for _, module := range application.EstimateModuleCatalog() {
		if values["module_"+module.ID] != "" {
			form.Add("estimate_module", module.ID)
		}
	}
	request, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	input, err := parseEstimateInput(request, application.EstimateInput{})
	return input, labels, err
}

// Small models sometimes omit an ID despite repeating its name in their reply.
// Resolve only a unique name explicitly present in the current user message.
func groundChatReferences(reply *estimateAssistantReply, message string, catalog chatCatalog) {
	if reply.Action == nil {
		return
	}
	find := func(items []chatItem) string {
		var id string
		for _, item := range items {
			if item.Name != "" && strings.Contains(strings.ToLower(message), strings.ToLower(item.Name)) {
				if id != "" {
					return ""
				}
				id = item.ID
			}
		}
		return id
	}
	if reply.Action.EquipmentID == "" {
		reply.Action.EquipmentID = find(catalog.Equipment)
	}
	if reply.Action.ProjectID == "" {
		reply.Action.ProjectID = find(catalog.Projects)
	}
	if reply.Action.EstimateID == "" {
		reply.Action.EstimateID = find(catalog.Estimates)
	}
	// Ground explicit fixed ruble fees in the user's words. This prevents a
	// small model from mistaking a payment amount for operation minutes.
	for _, field := range []struct{ role, key, timeKey string }{
		{"налад[а-яё]*", "fixed_setup_cost", "setup_time_minutes"},
		{"программ[а-яё]*", "fixed_programming_cost", "programming_hours"},
		{"оператор[а-яё]*", "fixed_operator_cost", ""},
	} {
		pattern := regexp.MustCompile("(?i)" + field.role + "(?:\\s+(?:с\\s+оплатой|оплата))?\\s+фикс[а-яё]*\\s+(\\d+(?:[.,]\\d+)?)\\s*(?:₽|руб)")
		match := pattern.FindStringSubmatch(message)
		if len(match) == 0 {
			continue
		}
		amount := json.Number(strings.ReplaceAll(match[1], ",", "."))
		if reply.Changes == nil {
			reply.Changes = map[string]json.Number{}
		}
		if reply.Changes[field.timeKey] == amount {
			delete(reply.Changes, field.timeKey)
		}
		reply.Changes[field.key] = amount
	}
}

func validateChatAction(reply estimateAssistantReply, state dialogState, catalog chatCatalog, values map[string]string) ([]string, error) {
	action := reply.Action
	if action == nil {
		return nil, application.Invalid("Уточните действие.", nil)
	}
	switch action.Operation {
	case "create_project":
		if strings.TrimSpace(action.Name) == "" {
			return nil, application.Invalid("Как назвать проект?", nil)
		}
		return []string{"Создать проект «" + action.Name + "»"}, nil
	case "select_project":
		name, err := chatItemName(catalog.Projects, action.ProjectID)
		return []string{"Перейти к проекту «" + name + "»"}, err
	case "select_estimate":
		name, err := chatItemName(catalog.Estimates, action.EstimateID)
		return []string{"Открыть смету «" + name + "»"}, err
	case "create_estimate", "update_estimate", "calculate":
		if action.Operation != "create_estimate" && state.EstimateID == "" {
			return nil, application.Invalid("Сначала выберите или создайте смету.", nil)
		}
		if action.Operation == "create_estimate" {
			projectID := action.ProjectID
			if projectID == "" {
				projectID = state.ProjectID
			}
			name, err := chatItemName(catalog.Projects, projectID)
			if err != nil {
				return nil, application.Invalid("В каком проекте создать смету? Укажите его название.", nil)
			}
			if strings.TrimSpace(action.Name) == "" {
				return nil, application.Invalid("Как назвать смету?", nil)
			}
			input, labels, err := prepareChatInput(reply, values, catalog)
			if err != nil {
				return nil, err
			}
			if input.Quantity <= 0 {
				return nil, application.Invalid("Укажите количество деталей.", nil)
			}
			if input.EquipmentID == "" {
				return nil, application.Invalid("Какое оборудование использовать? Укажите название станка компании.", nil)
			}
			labels = append([]string{fmt.Sprintf("Создать смету «%s», проект «%s», количество %d", action.Name, name, input.Quantity)}, labels...)
			equipmentName, _ := chatItemName(catalog.Equipment, input.EquipmentID)
			labels = append(labels, "Оборудование — "+equipmentName)
			return labels, nil
		}
		_, labels, err := prepareChatInput(reply, values, catalog)
		return append([]string{map[string]string{"update_estimate": "Сохранить параметры сметы", "calculate": "Пересчитать и сохранить версию"}[action.Operation]}, labels...), err
	default:
		return nil, application.Invalid("Это действие пока недоступно в чате.", nil)
	}
}

func (s *Server) executeChatAction(ctx context.Context, companyID string, state *dialogState, reply estimateAssistantReply, catalog chatCatalog, values map[string]string) (string, error) {
	if _, err := validateChatAction(reply, *state, catalog, maps.Clone(values)); err != nil {
		return "", err
	}
	action := reply.Action
	switch action.Operation {
	case "create_project":
		project, err := s.lifecycle.CreateProject(ctx, companyID, action.Name, "")
		if err == nil {
			state.ProjectID, state.EstimateID = project.ID, ""
		}
		return "Создан проект «" + project.Name + "». Теперь можно создать в нём смету.", err
	case "select_project":
		project, err := s.lifecycle.GetProject(ctx, companyID, action.ProjectID)
		if err == nil {
			state.ProjectID, state.EstimateID = project.ID, ""
		}
		return "Выбран проект «" + project.Name + "».", err
	case "select_estimate":
		doc, err := s.lifecycle.GetEstimateDocument(ctx, companyID, action.EstimateID)
		if err == nil {
			state.ProjectID, state.EstimateID = doc.Estimate.ProjectID, doc.Estimate.ID
		}
		return "Открыта смета «" + doc.Estimate.Name + "».", err
	case "create_estimate":
		projectID := action.ProjectID
		if projectID == "" {
			projectID = state.ProjectID
		}
		input, _, err := prepareChatInput(reply, values, catalog)
		if err != nil {
			return "", err
		}
		// Creation and all initial parameters are committed in one transaction.
		estimate, err := s.lifecycle.CreateManualEstimateInput(ctx, companyID, projectID, input)
		if err == nil {
			state.EstimateID, state.ProjectID = estimate.ID, estimate.ProjectID
		}
		return "Создана смета №" + estimate.Number + " «" + estimate.Name + "». Можно продолжать заполнять её здесь.", err
	case "update_estimate", "calculate":
		doc, err := s.lifecycle.GetEstimateDocument(ctx, companyID, state.EstimateID)
		if err != nil {
			return "", err
		}
		input, _, err := prepareChatInput(reply, values, catalog)
		if err != nil {
			return "", err
		}
		input.PartRevisionID, input.ProjectID = doc.Estimate.PartRevisionID, doc.Estimate.ProjectID
		if action.Operation == "update_estimate" {
			estimate, err := s.lifecycle.UpdateEstimateInput(ctx, companyID, state.EstimateID, input)
			return "Параметры сметы «" + estimate.Name + "» сохранены.", err
		}
		if input.MachineTime == nil {
			return "", application.Invalid("Укажите время обработки одной детали в минутах.", nil)
		}
		_, version, err := s.lifecycle.CalculateEstimate(ctx, companyID, state.EstimateID, input)
		if err != nil {
			return "", err
		}
		return "Сохранена версия " + strconv.Itoa(version.VersionNumber) + ". Цена партии " + formatMoney(version.FinalPrice, version.Currency) + ".", nil
	}
	return "", application.Invalid("Неизвестное действие.", nil)
}

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"smetchic/application"
)

type assistantField struct {
	key, label, unit string
}

var assistantFields = []assistantField{
	{"quantity", "Количество", "шт."},
	{"machine_time_hours", "Время обработки одной детали", "мин."},
	{"markup_percent", "Наценка", "%"},
	{"tool_usage_minutes", "Работа фрезы на деталь", "мин."},
	{"setup_time_minutes", "Наладка на партию", "мин."},
	{"programming_hours", "Программирование на партию", "ч."},
	{"material_weight_kg", "Масса заготовки на деталь", "кг"},
	{"fixed_operator_cost", "Оплата оператора на партию", ""},
	{"fixed_setup_cost", "Оплата наладки на партию", ""},
	{"fixed_programming_cost", "Оплата программирования на партию", ""},
}

type estimateAssistantReply struct {
	Reply         string                      `json:"reply"`
	Changes       map[string]json.Number      `json:"changes"`
	AddModules    application.EstimateModules `json:"add_modules"`
	RemoveModules application.EstimateModules `json:"remove_modules"`
	Action        *chatAction                 `json:"action,omitempty"`
	TextChanges   map[string]string           `json:"text_changes,omitempty"`
}

type chatAction struct {
	Operation   string `json:"operation"`
	Name        string `json:"name,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	EstimateID  string `json:"estimate_id,omitempty"`
	EquipmentID string `json:"equipment_id,omitempty"`
}

func askEstimateAssistant(ctx context.Context, message string, values map[string]string) (estimateAssistantReply, error) {
	model := strings.TrimSpace(os.Getenv("ESTIMATE_OLLAMA_MODEL"))
	if model == "" {
		model = "qwen3:4b"
	}
	return requestEstimateAssistant(ctx, "http://127.0.0.1:11434/api/chat", model, message, values)
}

func requestEstimateAssistant(ctx context.Context, endpoint, model, message string, values map[string]string) (estimateAssistantReply, error) {
	return requestDialogAssistant(ctx, endpoint, model, message, values, nil)
}

func requestDialogAssistant(ctx context.Context, endpoint, model, message string, values map[string]string, catalog any) (estimateAssistantReply, error) {
	current := make(map[string]string, len(assistantFields))
	properties := make(map[string]any, len(assistantFields))
	for _, field := range assistantFields {
		key := field.key
		if key == "machine_time_hours" {
			key = "machine_time_minutes"
		}
		current[key] = values[field.key]
		kind := "number"
		if field.key == "quantity" {
			kind = "integer"
		}
		properties[key] = map[string]any{
			"type": kind, "description": field.label + " " + field.unit,
		}
	}
	moduleIDs := []string{}
	moduleNames := map[string]string{}
	active := []string{}
	for _, module := range application.EstimateModuleCatalog() {
		moduleIDs = append(moduleIDs, module.ID)
		moduleNames[module.ID] = module.Name
		if values["module_"+module.ID] != "" {
			active = append(active, module.ID)
		}
	}
	moduleArray := map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": moduleIDs}}
	data, err := json.Marshal(map[string]any{"fields": current, "request": message, "modules": moduleNames, "active_modules": active, "catalog": catalog})
	if err != nil {
		return estimateAssistantReply{}, err
	}
	body, err := json.Marshal(map[string]any{
		"model": model, "stream": false, "think": false,
		"options": map[string]any{"temperature": 0, "num_predict": 512, "num_ctx": 4096},
		"format": map[string]any{
			"type": "object", "required": []string{"reply", "changes", "action"}, "additionalProperties": false,
			"properties": map[string]any{
				"reply":       map[string]any{"type": "string"},
				"changes":     map[string]any{"type": "object", "properties": properties, "additionalProperties": false},
				"add_modules": moduleArray, "remove_modules": moduleArray,
				"action": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation"},
					"properties": map[string]any{
						"operation": map[string]any{"type": "string", "enum": []string{"talk", "create_project", "select_project", "create_estimate", "select_estimate", "update_estimate", "calculate"}},
						"name":      map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"},
						"estimate_id": map[string]any{"type": "string"}, "equipment_id": map[string]any{"type": "string"},
					}},
				"text_changes": chatTextSchema(),
			},
		},
		"messages": []map[string]string{
			{"role": "system", "content": "Ты помощник приложения для производственных смет. Верни JSON по схеме. Ответ reply — кратко по-русски. " +
				"Текущий запрос пользователя — последняя строка request. Предыдущие реплики служат контекстом. " +
				"Выбери action.operation: create_project — создать проект, select_project — выбрать проект, create_estimate — создать смету, select_estimate — выбрать смету, update_estimate — изменить параметры, calculate — пересчитать, talk — обычный ответ или уточнение. " +
				"Для create_project укажи name из запроса. Для create_estimate нужны name, project_id, equipment_id, quantity. Если данных не хватает, выбери talk и спроси недостающее. " +
				"ID бери только из catalog. Для текущего проекта или сметы используй catalog.current_project_id и current_estimate_id. Не создавай вымышленные ID. " +
				"В changes включай только явно запрошенные числовые поля. Пропускай остальные ключи, даже если их значение нулевое. " +
				"machine_time_minutes — минуты обработки ОДНОЙ детали. programming_hours — часы программирования на партию. " +
				"Фиксированная оплата оператора — fixed_operator_cost, наладчика — fixed_setup_cost, программиста — fixed_programming_cost. Все суммы на ПАРТИЮ, в валюте сметы. " +
				"add_modules и remove_modules содержат ключи из modules. В text_changes укажи явно запрошенные названия, причины и выбранные ID. " +
				"Сервер выполнит предложенное действие после подтверждения. Не утверждай, что что-либо уже создано или сохранено. Итоговую стоимость вычисляет сервер. Не придумывай время и цены. Файл пока не анализируется."},
			{"role": "user", "content": string(data)},
		},
	})
	if err != nil {
		return estimateAssistantReply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return estimateAssistantReply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Do(req)
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return estimateAssistantReply{}, errors.New("Ollama не успел ответить за 90 секунд. Попробуйте короткий запрос или выберите более лёгкую локальную модель.")
		}
		return estimateAssistantReply{}, errors.New("Не удалось связаться с локальным Ollama. Проверьте, что он запущен.")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return estimateAssistantReply{}, fmt.Errorf("Модель не найдена. Загрузите её командой ollama pull %s.", model)
	}
	if res.StatusCode != http.StatusOK {
		return estimateAssistantReply{}, errors.New("Ollama не смог обработать запрос.")
	}
	var output struct {
		Message    struct{ Content string }
		DoneReason string `json:"done_reason"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&output); err != nil || output.DoneReason == "length" {
		return estimateAssistantReply{}, errors.New("Модель вернула неполный ответ. Попробуйте более короткий запрос.")
	}
	var reply estimateAssistantReply
	if err := json.Unmarshal([]byte(output.Message.Content), &reply); err != nil || strings.TrimSpace(reply.Reply) == "" {
		return estimateAssistantReply{}, errors.New("Модель вернула ответ в неожиданном формате.")
	}
	if minutes, ok := reply.Changes["machine_time_minutes"]; ok {
		delete(reply.Changes, "machine_time_minutes")
		reply.Changes["machine_time_hours"] = minutes
	}
	return reply, nil
}

func assistantFormChanges(values map[string]string, reply estimateAssistantReply) ([]string, error) {
	updates := make(map[string]string, len(reply.Changes))
	var labels []string
	for _, field := range assistantFields {
		number, ok := reply.Changes[field.key]
		if !ok {
			continue
		}
		value, err := number.Float64()
		if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("Модель предложила некорректное значение для поля «%s».", field.label)
		}
		formatted := strconv.FormatFloat(value, 'f', -1, 64)
		if field.key == "quantity" {
			count, err := strconv.Atoi(formatted)
			if err != nil || count <= 0 {
				return nil, errors.New("Модель предложила некорректное количество деталей.")
			}
		}
		updates[field.key] = formatted
		labels = append(labels, strings.TrimSpace(field.label+" → "+formatted+" "+field.unit))
	}
	if len(updates) != len(reply.Changes) {
		return nil, errors.New("Модель предложила изменение поля, недоступного помощнику.")
	}
	next := maps.Clone(values)
	moduleLabels, err := applyAssistantModuleChanges(next, reply, updates)
	if err != nil {
		return nil, err
	}
	labels = append(labels, moduleLabels...)
	for key, value := range updates {
		next[key] = value
	}
	if _, changed := updates["markup_percent"]; changed {
		next["markup_source"] = "user_override"
		next["markup_reason"] = ""
	}
	if _, changed := updates["machine_time_hours"]; changed {
		next["machine_time_reason"] = ""
	}
	for _, field := range []struct{ key, reason string }{
		{"tool_usage_minutes", "tool_usage_reason"},
		{"setup_time_minutes", "setup_reason"},
		{"programming_hours", "programming_reason"},
	} {
		if _, changed := updates[field.key]; changed {
			next[field.reason] = ""
		}
	}
	clear(values)
	maps.Copy(values, next)
	return labels, nil
}

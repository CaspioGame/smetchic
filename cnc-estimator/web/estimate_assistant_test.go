package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smetchic/application"
)

func TestAssistantRequestsStructuredLocalReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string
			Think    bool
			Stream   bool
			Format   json.RawMessage
			Messages []struct{ Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || r.Method != http.MethodPost ||
			request.Model != "qwen3:4b" || request.Think || request.Stream || len(request.Format) == 0 ||
			len(request.Messages) != 2 || !strings.Contains(request.Messages[1].Content, "20 деталей") ||
			strings.Contains(request.Messages[1].Content, "private-equipment-id") {
			t.Errorf("unexpected model request: %#v, err = %v", request, err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": `{"reply":"Предлагаю 20 деталей.","changes":{"quantity":20}}`},
		})
	}))
	defer server.Close()
	reply, err := requestEstimateAssistant(context.Background(), server.URL, "qwen3:4b", "Поставь 20 деталей",
		map[string]string{"quantity": "1", "equipment_id": "private-equipment-id"})
	if err != nil || reply.Changes["quantity"] != "20" {
		t.Fatalf("reply = %#v, err = %v", reply, err)
	}
}

func TestAssistantCanReplaceMaterialAndRemoveWorkerModule(t *testing.T) {
	values := map[string]string{
		"module_stock_material": "1", "material_id": "steel", "material_weight_kg": "2",
		"module_programming": "1", "fixed_programming_cost": "1250",
	}
	labels, err := assistantFormChanges(values, estimateAssistantReply{
		AddModules:    application.EstimateModules{application.ModuleCatalogMaterial},
		RemoveModules: application.EstimateModules{application.ModuleProgramming},
	})
	if err != nil || len(labels) != 3 || values["module_catalog_material"] != "1" ||
		values["module_stock_material"] != "" || values["material_id"] != "" ||
		values["module_programming"] != "" || values["fixed_programming_cost"] != "" {
		t.Fatalf("module proposal = %v, labels = %v, err = %v", values, labels, err)
	}
}

func TestAssistantFormChanges(t *testing.T) {
	values := map[string]string{"quantity": "1", "equipment_id": "machine", "markup_reason": "Старая причина"}
	reply := estimateAssistantReply{Changes: map[string]json.Number{
		"quantity": "20", "machine_time_hours": "19.8", "markup_percent": "50", "fixed_setup_cost": "0",
	}}
	labels, err := assistantFormChanges(values, reply)
	if err != nil || len(labels) != 5 || values["quantity"] != "20" || values["machine_time_hours"] != "19.8" ||
		values["fixed_setup_cost"] != "0" || values["markup_source"] != "user_override" ||
		values["markup_reason"] != "" || values["equipment_id"] != "machine" || values["module_setup"] != "1" {
		t.Fatalf("values = %v, labels = %v, err = %v", values, labels, err)
	}
	reply.Changes = map[string]json.Number{"quantity": "30", "equipment_id": "123"}
	if _, err := assistantFormChanges(values, reply); err == nil || values["quantity"] != "20" {
		t.Fatalf("invalid response modified the form: values = %v, err = %v", values, err)
	}
}

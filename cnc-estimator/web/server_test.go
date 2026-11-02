package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smetchic/equipment"
	"smetchic/model"
)

const validPart = `[part]
material=aluminum
stock_width=40
stock_length=30
stock_height=20

[feature hole-1]
type=hole
x=10
y=12
diameter=5
depth=8
`

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	catalog, err := equipment.NewCatalog([]model.EquipmentModel{
		{
			ID: "dekart-s4040", Type: model.EquipmentMilling,
			Manufacturer: "DeKart", Model: "S4040", MaxFeed: 5000,
		},
		{
			ID: "test-mill-model", Type: model.EquipmentMilling,
			Manufacturer: "Test", Model: "TM-1", MaxFeed: 5000,
		},
	}, []model.Equipment{
		{
			ID: "dekart-s4040", CompanyID: "test-company", EquipmentModelID: "dekart-s4040",
			Name: "DeKart S4040", Status: model.EquipmentStatusActive, MachineRatePerHour: 1500,
		},
		{
			ID: "test-mill", CompanyID: "test-company", EquipmentModelID: "test-mill-model",
			Name: "Test Mill", Status: model.EquipmentStatusActive, MachineRatePerHour: 3000,
		},
	})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	handler, err := NewHandler(catalog, model.EstimateSettings{
		EquipmentID:   "dekart-s4040",
		MarkupPercent: 50,
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func TestIndexPage(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, expected := range []string{
		"CNC Estimator",
		`method="POST"`,
		`enctype="multipart/form-data"`,
		`name="part_file"`,
		`name="equipment_id"`,
		`value="dekart-s4040"`,
		`Test Mill`,
		`value="test-mill"`,
		`value="50"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("GET / body does not contain %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), `name="machine_rate_per_hour"`) {
		t.Error("equipment rate must come from the catalog, not a form field")
	}
}

func TestUploadDisplaysEstimate(t *testing.T) {
	request := uploadRequest(t, "square.part", validPart)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST / status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, expected := range []string{
		"PART", "square.part", "FEATURES", "hole-1", "ОБОРУДОВАНИЕ", "DeKart S4040",
		"Фрезерный станок", "DeKart", "S4040", "1500.00 ₽/ч",
		"OPERATIONS", "Drilling", "Drill", "Estimated/test toolpath length",
		"TIME", "COST", "ESTIMATE", "Итоговая цена (demo markup)",
		"НАСТРОЙКИ РАСЧЁТА", "Ставка", "Наценка",
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("result body does not contain %q", expected)
		}
	}
}

func TestUploadUsesSelectedEquipmentAndRate(t *testing.T) {
	tests := []struct {
		name        string
		equipmentID string
		wantRate    string
		wantMarkup  string
		wantCost    string
		wantPrice   string
	}{
		{
			name:        "DeKart equipment",
			equipmentID: "dekart-s4040",
			wantRate:    "1500.00 ₽/ч",
			wantMarkup:  "50.00%",
			wantCost:    "3.00 ₽",
			wantPrice:   "4.50 ₽",
		},
		{
			name:        "second equipment uses its catalog rate",
			equipmentID: "test-mill",
			wantRate:    "3000.00 ₽/ч",
			wantMarkup:  "50.00%",
			wantCost:    "6.00 ₽",
			wantPrice:   "9.00 ₽",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := uploadRequestWithEquipment(t, "settings.part", validPart, test.equipmentID, "50")
			response := httptest.NewRecorder()
			testHandler(t).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("POST / status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
			}
			for _, expected := range []string{test.wantRate, test.wantMarkup, test.wantCost, test.wantPrice} {
				if !strings.Contains(response.Body.String(), expected) {
					t.Errorf("result body does not contain %q", expected)
				}
			}
		})
	}
}

func TestUploadValidatesEstimateSettings(t *testing.T) {
	tests := []struct {
		name        string
		equipmentID string
		markup      string
		wantError   string
	}{
		{name: "unknown equipment", equipmentID: "missing", markup: "50", wantError: "оборудование с ID"},
		{name: "empty equipment ID", equipmentID: "", markup: "50", wantError: "оборудование с ID"},
		{name: "empty markup", equipmentID: "dekart-s4040", markup: "", wantError: "Заполните поле «Наценка»"},
		{name: "invalid markup", equipmentID: "dekart-s4040", markup: "NaN", wantError: "корректное число"},
		{name: "negative markup", equipmentID: "dekart-s4040", markup: "-1", wantError: "не может быть отрицательной"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := uploadRequestWithEquipment(t, "settings.part", validPart, test.equipmentID, test.markup)
			response := httptest.NewRecorder()
			testHandler(t).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("POST / status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if !strings.Contains(response.Body.String(), test.wantError) {
				t.Errorf("error page does not contain %q: %s", test.wantError, response.Body.String())
			}
		})
	}
}

func TestUploadShowsParserError(t *testing.T) {
	invalidPart := strings.Replace(validPart, "stock_width=40", "stock_width=bad", 1)
	request := uploadRequest(t, "invalid.part", invalidPart)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST / invalid part status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	for _, expected := range []string{"Ошибка расчёта", "invalid.part", "stock_width", "положительным числом"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("error body does not contain %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), "goroutine") {
		t.Error("error page must not include a stack trace")
	}
}

func TestUploadRequiresFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("equipment_id", "dekart-s4040"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("markup_percent", "50"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST / without file status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "Выберите файл .part") {
		t.Errorf("missing file error not shown: %s", response.Body.String())
	}
}

func uploadRequest(t *testing.T, filename, content string) *http.Request {
	return uploadRequestWithEquipment(t, filename, content, "dekart-s4040", "50")
}

func uploadRequestWithEquipment(t *testing.T, filename, content, equipmentID, markup string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"equipment_id": equipmentID, "markup_percent": markup} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("part_file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

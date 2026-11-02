package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"smetchic/model"
	"smetchic/storage"
)

var csrfInputPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func newTestCabinet(t *testing.T) (*storage.Store, http.Handler) {
	t.Helper()
	store, err := storage.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	handler, err := NewCabinetHandler(store, storage.DemoCompanyID)
	if err != nil {
		t.Fatalf("NewCabinetHandler() error = %v", err)
	}
	return store, handler
}

func TestCabinetGETRoutesRenderNavigation(t *testing.T) {
	_, handler := newTestCabinet(t)
	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "Расчёт сметы"},
		{path: "/equipment", want: "DeKart S4040"},
		{path: "/equipment/new", want: "Новое оборудование"},
		{path: "/materials", want: "Материалы пока не добавлены"},
		{path: "/materials/new", want: "Новый материал"},
		{path: "/stock-blanks", want: "Заготовки пока не добавлены"},
		{path: "/stock-blanks/new", want: "Прямоугольная"},
		{path: "/company", want: "Демонстрационная компания"},
		{path: "/company?saved=1", want: "Изменения сохранены."},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d: %s", test.path, response.Code, http.StatusOK, response.Body.String())
			}
			for _, expected := range []string{test.want, `href="/equipment"`, `href="/materials"`, `href="/stock-blanks"`, `href="/company"`} {
				if !strings.Contains(response.Body.String(), expected) {
					t.Errorf("GET %s response does not contain %q", test.path, expected)
				}
			}
			if len(response.Result().Cookies()) == 0 {
				t.Error("GET page did not set a CSRF cookie")
			}
		})
	}
}

func TestEquipmentCRUDAndCompanyScope(t *testing.T) {
	store, handler := newTestCabinet(t)
	token, cookie := cabinetCSRF(t, handler, "/equipment/new")

	create := url.Values{
		"csrf_token":            {token},
		"id":                    {""},
		"name":                  {"Фрезер №2"},
		"equipment_model_id":    {"dekart-s4040"},
		"inventory_number":      {"INV-2"},
		"serial_number":         {"SER-2"},
		"status":                {"active"},
		"machine_rate_per_hour": {"2200"},
		"company_id":            {"attacker-company"},
	}
	response := cabinetPost(t, handler, "/equipment/save", create, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create equipment status = %d: %s", response.Code, response.Body.String())
	}

	snapshot, err := store.Snapshot(storage.DemoCompanyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Equipment) != 2 {
		t.Fatalf("equipment count = %d, want 2", len(snapshot.Equipment))
	}
	var created model.Equipment
	for _, item := range snapshot.Equipment {
		if item.Name == "Фрезер №2" {
			created = item
		}
	}
	if created.ID == "" || created.CompanyID != storage.DemoCompanyID ||
		created.MachineRatePerHour != 2200 || created.EquipmentModelID != "dekart-s4040" {
		t.Fatalf("equipment did not retain server-scoped fields: %+v", created)
	}

	update := url.Values{
		"csrf_token": {token}, "id": {created.ID}, "name": {"Фрезер №2 — обновлён"},
		"equipment_model_id": {"dekart-s4040"}, "status": {"maintenance"},
		"machine_rate_per_hour": {"2500"},
	}
	response = cabinetPost(t, handler, "/equipment/save", update, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("update equipment status = %d: %s", response.Code, response.Body.String())
	}
	updated, err := store.GetEquipment(storage.DemoCompanyID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Фрезер №2 — обновлён" || updated.MachineRatePerHour != 2500 ||
		updated.Status != model.EquipmentStatusMaintenance {
		t.Fatalf("equipment was not updated: %+v", updated)
	}

	remove := url.Values{"csrf_token": {token}, "id": {created.ID}}
	response = cabinetPost(t, handler, "/equipment/delete", remove, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("delete equipment status = %d: %s", response.Code, response.Body.String())
	}
	if _, err := store.GetEquipment(storage.DemoCompanyID, created.ID); err == nil {
		t.Fatal("equipment still exists after delete")
	}
	snapshot, err = store.Snapshot(storage.DemoCompanyID)
	if err != nil || len(snapshot.EquipmentModels) != 1 {
		t.Fatalf("deleting equipment must preserve its model; snapshot=%+v err=%v", snapshot, err)
	}
}

func TestMaterialAndStockBlankCRUDAndReferences(t *testing.T) {
	store, handler := newTestCabinet(t)
	token, cookie := cabinetCSRF(t, handler, "/materials/new")

	materialForm := url.Values{
		"csrf_token": {token}, "name": {"Алюминий АМг6"}, "grade": {"АМг6"},
		"category": {"aluminum"}, "density": {"2700"}, "price_per_kg": {"100"},
		"currency": {"RUB"}, "company_id": {"not-trusted"},
	}
	response := cabinetPost(t, handler, "/materials/save", materialForm, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create material status = %d: %s", response.Code, response.Body.String())
	}
	snapshot, err := store.Snapshot(storage.DemoCompanyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Materials) != 1 || snapshot.Materials[0].CompanyID != storage.DemoCompanyID {
		t.Fatalf("material was not saved in current company: %+v", snapshot.Materials)
	}
	material := snapshot.Materials[0]
	updatedMaterialForm := url.Values{
		"csrf_token": {token}, "id": {material.ID}, "name": {"Алюминий АМг6 обновлённый"},
		"grade": {"АМг6"}, "category": {"aluminum"}, "density": {"2700"},
		"price_per_kg": {"120"}, "currency": {"RUB"},
	}
	response = cabinetPost(t, handler, "/materials/save", updatedMaterialForm, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("update material status = %d: %s", response.Code, response.Body.String())
	}
	material, err = findMaterial(store, storage.DemoCompanyID, material.ID)
	if err != nil || material.Name != "Алюминий АМг6 обновлённый" || material.PricePerKg != 120 {
		t.Fatalf("material was not updated: %+v err=%v", material, err)
	}
	materialList := httptest.NewRecorder()
	handler.ServeHTTP(materialList, httptest.NewRequest(http.MethodGet, "/materials", nil))
	for _, expected := range []string{"Алюминий АМг6 обновлённый", "Алюминий", "120.00"} {
		if !strings.Contains(materialList.Body.String(), expected) {
			t.Errorf("material listing does not contain %q", expected)
		}
	}

	blankForm := url.Values{
		"csrf_token": {token}, "name": {"Плита 100×50×20"}, "material_id": {material.ID},
		"shape": {"rectangular"}, "length": {"100"}, "width": {"50"}, "height": {"20"},
		"currency": {"RUB"}, "price_per_piece": {""},
	}
	response = cabinetPost(t, handler, "/stock-blanks/save", blankForm, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create stock blank status = %d: %s", response.Code, response.Body.String())
	}
	snapshot, err = store.Snapshot(storage.DemoCompanyID)
	if err != nil || len(snapshot.StockBlanks) != 1 {
		t.Fatalf("stock blank was not saved: %+v err=%v", snapshot.StockBlanks, err)
	}
	blank := snapshot.StockBlanks[0]
	if blank.MaterialID != material.ID || blank.CompanyID != storage.DemoCompanyID {
		t.Fatalf("stock blank is linked to wrong company/material: %+v", blank)
	}
	updatedBlankForm := url.Values{
		"csrf_token": {token}, "id": {blank.ID}, "name": {"Плита обновлённая"},
		"material_id": {material.ID}, "shape": {"rectangular"}, "length": {"120"},
		"width": {"50"}, "height": {"20"}, "currency": {"RUB"}, "price_per_piece": {""},
	}
	response = cabinetPost(t, handler, "/stock-blanks/save", updatedBlankForm, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("update stock blank status = %d: %s", response.Code, response.Body.String())
	}
	snapshot, err = store.Snapshot(storage.DemoCompanyID)
	if err != nil || len(snapshot.StockBlanks) != 1 || snapshot.StockBlanks[0].Length != 120 {
		t.Fatalf("stock blank was not updated: %+v err=%v", snapshot.StockBlanks, err)
	}
	blank = snapshot.StockBlanks[0]

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/stock-blanks", nil))
	for _, expected := range []string{"0.324 кг", "38.88", "120 × 50 × 20"} {
		if !strings.Contains(list.Body.String(), expected) {
			t.Errorf("stock blank listing does not contain %q", expected)
		}
	}

	removeMaterial := url.Values{"csrf_token": {token}, "id": {material.ID}}
	response = cabinetPost(t, handler, "/materials/delete", removeMaterial, cookie)
	if response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "используется одной или несколькими заготовками") {
		t.Fatalf("referenced material deletion response = %d: %s", response.Code, response.Body.String())
	}

	crossCompanyBlank := url.Values{
		"csrf_token": {token}, "name": {"Чужая"}, "material_id": {"another-company-material"},
		"shape": {"round"}, "diameter": {"20"}, "length_round": {"100"},
		"currency": {"RUB"}, "price_per_piece": {"10"},
	}
	response = cabinetPost(t, handler, "/stock-blanks/save", crossCompanyBlank, cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown/cross-company material status = %d", response.Code)
	}

	response = cabinetPost(t, handler, "/stock-blanks/delete", url.Values{"csrf_token": {token}, "id": {blank.ID}}, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("delete stock blank status = %d: %s", response.Code, response.Body.String())
	}
	response = cabinetPost(t, handler, "/materials/delete", removeMaterial, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("delete now-unreferenced material status = %d: %s", response.Code, response.Body.String())
	}
}

func TestCompanySettingsSaveAndCSRFProtection(t *testing.T) {
	store, handler := newTestCabinet(t)
	token, cookie := cabinetCSRF(t, handler, "/company")
	form := url.Values{
		"csrf_token": {token}, "name": {"Моя мастерская"},
		"currency": {"EUR"}, "default_markup_percent": {"75"},
	}
	response := cabinetPost(t, handler, "/company/save", form, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save company settings status = %d: %s", response.Code, response.Body.String())
	}
	snapshot, err := store.Snapshot(storage.DemoCompanyID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Company.Name != "Моя мастерская" || snapshot.Settings.Currency != "EUR" ||
		snapshot.Settings.DefaultMarkupPercent != 75 {
		t.Fatalf("company settings not saved: %+v %+v", snapshot.Company, snapshot.Settings)
	}

	request := httptest.NewRequest(http.MethodPost, "/company/save", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF token status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestCabinetEstimateStillUsesPersistedEquipment(t *testing.T) {
	_, handler := newTestCabinet(t)
	request := uploadRequestWithEquipment(t, "square.part", validPart, "dekart-s4040", "50")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("estimate POST status = %d: %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{"DeKart S4040", "1500.00 ₽/ч", "Итоговая цена (demo markup)", "COST", "TIME"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("estimate result does not contain %q", expected)
		}
	}
}

func TestCabinetEstimateRejectsUnknownEquipment(t *testing.T) {
	_, handler := newTestCabinet(t)
	request := uploadRequestWithEquipment(t, "square.part", validPart, "not-in-company", "50")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "оборудование с ID") {
		t.Fatalf("unknown equipment result = %d: %s", response.Code, response.Body.String())
	}
}

func TestStockBlankRoundFormUsesRoundDimensions(t *testing.T) {
	store, handler := newTestCabinet(t)
	material := model.Material{
		ID: "steel-45", CompanyID: storage.DemoCompanyID, Name: "Сталь 45",
		Grade: "45", Category: model.MaterialCategorySteel, Density: 8000, PricePerKg: 80, Currency: "RUB",
	}
	if err := store.SaveMaterial(storage.DemoCompanyID, material); err != nil {
		t.Fatal(err)
	}
	token, cookie := cabinetCSRF(t, handler, "/stock-blanks/new")
	form := url.Values{
		"csrf_token": {token}, "name": {"Круг Ø20"}, "material_id": {material.ID},
		"shape": {"round"}, "diameter": {"20"}, "length_round": {"100"},
		"currency": {"RUB"}, "price_per_piece": {""},
	}
	response := cabinetPost(t, handler, "/stock-blanks/save", form, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save round blank status = %d: %s", response.Code, response.Body.String())
	}
	snapshot, err := store.Snapshot(storage.DemoCompanyID)
	if err != nil || len(snapshot.StockBlanks) != 1 {
		t.Fatalf("round blank not saved: %+v err=%v", snapshot.StockBlanks, err)
	}
	volume, err := snapshot.StockBlanks[0].VolumeMm3()
	if err != nil || volume < 31415 || volume > 31416 {
		t.Fatalf("round volume = %v err=%v, want approximately 31415.9", volume, err)
	}
}

func TestStockBlankWithoutPriceShowsUnavailableCost(t *testing.T) {
	store, handler := newTestCabinet(t)
	material := model.Material{
		ID: "unpriced-aluminum", CompanyID: storage.DemoCompanyID, Name: "Алюминий без цены",
		Grade: "TEST", Category: model.MaterialCategoryAluminum,
		Density: 2700, PricePerKg: 0, Currency: "RUB",
	}
	if err := store.SaveMaterial(storage.DemoCompanyID, material); err != nil {
		t.Fatal(err)
	}
	blank := model.StockBlank{
		ID: "unpriced-blank", CompanyID: storage.DemoCompanyID, MaterialID: material.ID,
		Name: "Заготовка без цены", Shape: model.StockBlankShapeRectangular,
		Length: 100, Width: 50, Height: 20, Currency: "RUB",
	}
	if err := store.SaveStockBlank(storage.DemoCompanyID, blank); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/stock-blanks", nil))
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Не рассчитано") ||
		!strings.Contains(response.Body.String(), "Укажите цену за заготовку") {
		t.Fatalf("missing material price should be explicit, got %d: %s", response.Code, response.Body.String())
	}
}

func cabinetCSRF(t *testing.T, handler http.Handler, path string) (string, *http.Cookie) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	match := csrfInputPattern.FindStringSubmatch(response.Body.String())
	if len(match) != 2 {
		t.Fatalf("CSRF input not found on GET %s: %s", path, response.Body.String())
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "cnc_csrf" {
			return match[1], cookie
		}
	}
	t.Fatalf("CSRF cookie not set on GET %s", path)
	return "", nil
}

func cabinetPost(t *testing.T, handler http.Handler, path string, values url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func multipartEstimateRequest(t *testing.T, equipmentID, markup string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("equipment_id", equipmentID); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("markup_percent", markup); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("part_file", "square.part")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(validPart)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

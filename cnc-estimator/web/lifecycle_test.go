package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"smetchic/application"
	"smetchic/model"
	"smetchic/storage"
)

type lifecycleTestRepository struct {
	application.Repository
	projects           []application.ProjectRecord
	project            application.ProjectRecord
	part               application.PartRecord
	revision           application.PartRevisionRecord
	estimate           application.EstimateRecord
	version            application.EstimateVersionRecord
	versionCreateCalls int
}

func (repository *lifecycleTestRepository) ListProjects(context.Context, string) ([]application.ProjectRecord, error) {
	return repository.projects, nil
}

func (repository *lifecycleTestRepository) CreateProject(_ context.Context, project application.ProjectRecord) error {
	repository.project = project
	repository.projects = append(repository.projects, project)
	return nil
}

func (repository *lifecycleTestRepository) UpdateProject(_ context.Context, project application.ProjectRecord) (application.ProjectRecord, error) {
	if repository.project.ID != project.ID || repository.project.CompanyID != project.CompanyID {
		return application.ProjectRecord{}, application.ErrNotFound
	}
	repository.project.Name, repository.project.Description = project.Name, project.Description
	return repository.project, nil
}

func (repository *lifecycleTestRepository) GetProject(_ context.Context, companyID, projectID string) (application.ProjectRecord, error) {
	if repository.project.ID == projectID && repository.project.CompanyID == companyID {
		return repository.project, nil
	}
	return application.ProjectRecord{}, application.ErrNotFound
}

func (repository *lifecycleTestRepository) CreatePartWithRevision(_ context.Context, part application.PartRecord, revision application.PartRevisionRecord) (application.PartRecord, application.PartRevisionRecord, error) {
	revision.RevisionNumber = 1
	repository.part, repository.revision = part, revision
	return part, revision, nil
}

func (repository *lifecycleTestRepository) GetRevisionEstimate(_ context.Context, companyID, revisionID string) (application.RevisionEstimate, error) {
	if repository.revision.ID != revisionID || repository.revision.CompanyID != companyID {
		return application.RevisionEstimate{}, application.ErrNotFound
	}
	return application.RevisionEstimate{
		Project: repository.project, Part: repository.part, Revision: repository.revision,
	}, nil
}

func (repository *lifecycleTestRepository) CreateEstimateVersion(_ context.Context, estimate application.EstimateRecord, version application.EstimateVersionRecord) (application.EstimateRecord, application.EstimateVersionRecord, error) {
	repository.versionCreateCalls++
	estimate.ID = "estimate_test"
	estimate.CreatedAt = time.Now()
	version.EstimateID = estimate.ID
	version.ID = "version_test"
	version.VersionNumber = 1
	repository.estimate, repository.version = estimate, version
	return estimate, version, nil
}

func (repository *lifecycleTestRepository) GetEstimateVersion(_ context.Context, companyID, estimateID, versionID string) (application.EstimateVersionDetail, error) {
	if repository.estimate.ID != estimateID || repository.estimate.CompanyID != companyID ||
		repository.version.ID != versionID || repository.version.EstimateID != estimateID {
		return application.EstimateVersionDetail{}, application.ErrNotFound
	}
	return application.EstimateVersionDetail{Estimate: repository.estimate, Version: repository.version}, nil
}

func (repository *lifecycleTestRepository) ListEstimateVersions(_ context.Context, companyID, estimateID string) ([]application.EstimateVersionRecord, error) {
	if repository.estimate.ID != estimateID || repository.estimate.CompanyID != companyID {
		return nil, application.ErrNotFound
	}
	return []application.EstimateVersionRecord{repository.version}, nil
}

func (repository *lifecycleTestRepository) CreateEstimate(_ context.Context, estimate application.EstimateRecord) (application.EstimateRecord, error) {
	repository.estimate = estimate
	return estimate, nil
}

func (repository *lifecycleTestRepository) UpdateEstimateInput(_ context.Context, _ string, estimate application.EstimateRecord) (application.EstimateRecord, error) {
	repository.estimate = estimate
	return estimate, nil
}

func (repository *lifecycleTestRepository) ListCompanyEstimates(_ context.Context, companyID string) ([]application.EstimateRecord, error) {
	if repository.estimate.CompanyID != companyID || repository.estimate.ID == "" {
		return nil, nil
	}
	return []application.EstimateRecord{repository.estimate}, nil
}

func (repository *lifecycleTestRepository) GetEstimateDocument(_ context.Context, companyID, estimateID string) (application.EstimateDocument, error) {
	if repository.estimate.ID != estimateID || repository.estimate.CompanyID != companyID {
		return application.EstimateDocument{}, application.ErrNotFound
	}
	return application.EstimateDocument{
		Estimate: repository.estimate, Project: repository.project, Part: repository.part,
		Revision: repository.revision, Versions: []application.EstimateVersionRecord{repository.version},
	}, nil
}

func (repository *lifecycleTestRepository) GetEstimate(_ context.Context, companyID, estimateID string) (application.EstimateRecord, error) {
	if repository.estimate.ID != estimateID || repository.estimate.CompanyID != companyID {
		return application.EstimateRecord{}, application.ErrNotFound
	}
	return repository.estimate, nil
}

func TestOpeningHistoricalEstimateDoesNotRecalculate(t *testing.T) {
	handler, repository := newLifecycleTestHandler(t)
	companyID := storage.DemoCompanyID
	repository.estimate = application.EstimateRecord{
		ID: "estimate_history", CompanyID: companyID, Number: "S-12", Name: "Historical estimate",
		CurrentVersionID: "version_history",
	}
	repository.version = application.EstimateVersionRecord{
		ID: "version_history", CompanyID: companyID, EstimateID: repository.estimate.ID,
		VersionNumber: 1, Currency: "RUB",
		InputSnapshot:    []byte(`{"WorkingInput":{"Quantity":20}}`),
		SettingsSnapshot: []byte(`{"GlobalMaterial":{"Name":"Алюминий","Grade":"6061"}}`),
		ResultSnapshot:   []byte(`{"CostBreakdown":{"Quantity":20,"Currency":"RUB","MaterialCost":200,"MaterialPriceSource":"global_reference_price","TotalCost":250,"TotalPerPieceCost":12.5,"SellingPrice":375,"SellingPricePerPiece":18.75,"MarkupPercent":50}}`),
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/estimates/"+repository.estimate.ID+"/versions/"+repository.version.ID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("historical version status = %d, body: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Алюминий 6061") ||
		!strings.Contains(response.Body.String(), "Материал") ||
		!strings.Contains(response.Body.String(), "250.00 ₽") {
		t.Fatalf("historical snapshot values were not rendered: %s", response.Body.String())
	}
	if repository.versionCreateCalls != 0 {
		t.Fatalf("opening a historical version recalculated it %d times", repository.versionCreateCalls)
	}
}

func (repository *lifecycleTestRepository) GetCompany(_ context.Context, companyID string) (model.Company, error) {
	return model.Company{ID: companyID, Name: "Demo company"}, nil
}

func (repository *lifecycleTestRepository) GetCompanySettings(_ context.Context, companyID string) (model.CompanySettings, error) {
	return model.CompanySettings{CompanyID: companyID, Currency: "RUB", DefaultMarkupPercent: 50}, nil
}

func (repository *lifecycleTestRepository) GetEquipment(_ context.Context, companyID, equipmentID string) (model.Equipment, error) {
	return testLifecycleEquipment(companyID, equipmentID), nil
}

func (repository *lifecycleTestRepository) ListEquipment(_ context.Context, companyID string) ([]model.Equipment, error) {
	return []model.Equipment{testLifecycleEquipment(companyID, "equipment_test")}, nil
}

func (repository *lifecycleTestRepository) ListMaterials(context.Context, string) ([]model.Material, error) {
	return nil, nil
}

func (repository *lifecycleTestRepository) ListGlobalMaterials(context.Context) ([]model.GlobalMaterial, error) {
	return []model.GlobalMaterial{{ID: "catalog-material", Name: "Сталь", ReferencePriceKg: 200, Currency: "RUB"}}, nil
}

func (repository *lifecycleTestRepository) ListGlobalTools(context.Context) ([]model.Consumable, error) {
	return []model.Consumable{{ID: "catalog-tool", Name: "Фреза", UnitPrice: 1200, ResourceMinutes: 120, Currency: "RUB"}}, nil
}

func (repository *lifecycleTestRepository) ListStockBlanks(context.Context, string) ([]model.StockBlank, error) {
	return nil, nil
}

func (repository *lifecycleTestRepository) GetCompanyEconomics(_ context.Context, companyID string) (model.CompanyEconomics, error) {
	return model.CompanyEconomics{
		CompanyID: companyID, RentAllocationMethod: model.AllocationByProductionHours, Currency: "RUB",
	}, nil
}

func (repository *lifecycleTestRepository) SaveCompanyEconomics(context.Context, model.CompanyEconomics) error {
	return nil
}

func (repository *lifecycleTestRepository) ListEquipmentEconomics(context.Context, string) ([]model.EquipmentEconomics, error) {
	return nil, nil
}

func (repository *lifecycleTestRepository) GetEquipmentEconomics(context.Context, string, string) (*model.EquipmentEconomics, error) {
	return nil, nil
}

func (repository *lifecycleTestRepository) SaveEquipmentEconomics(context.Context, model.EquipmentEconomics) error {
	return nil
}

func (repository *lifecycleTestRepository) DeleteEquipmentEconomics(context.Context, string, string) error {
	return nil
}

func (repository *lifecycleTestRepository) ListPersonnelRates(context.Context, string) ([]model.PersonnelRate, error) {
	return []model.PersonnelRate{{Role: "operator", MonthlyCost: 80000, MonthlyAvailableHours: 160, Currency: "RUB"}}, nil
}

func (repository *lifecycleTestRepository) SavePersonnelRate(context.Context, model.PersonnelRate) error {
	return nil
}

func (repository *lifecycleTestRepository) DeletePersonnelRate(context.Context, string, string) error {
	return nil
}

func (repository *lifecycleTestRepository) ListCompanyOverheads(context.Context, string) ([]model.CompanyOverhead, error) {
	return nil, nil
}

func (repository *lifecycleTestRepository) SaveCompanyOverhead(context.Context, model.CompanyOverhead) error {
	return nil
}

func (repository *lifecycleTestRepository) DeleteCompanyOverhead(context.Context, string, string) error {
	return nil
}

func (repository *lifecycleTestRepository) ListConsumables(context.Context, string) ([]model.Consumable, error) {
	return []model.Consumable{{
		ID: "stock-tool", Name: "Складская фреза", StockQuantity: 1,
		UnitPrice: 1000, ResourceMinutes: 120, UsedMinutesByUnit: []float64{70}, Currency: "RUB",
	}}, nil
}

func (repository *lifecycleTestRepository) ListOrders(context.Context, string) ([]application.OrderRecord, error) {
	return nil, nil
}

func testLifecycleEquipment(companyID, equipmentID string) model.Equipment {
	return model.Equipment{
		ID: equipmentID, CompanyID: companyID, EquipmentModelID: "model_test",
		Name: "Test CNC", Status: model.EquipmentStatusActive, MachineRatePerHour: 1500,
		EquipmentModel: model.EquipmentModel{
			ID: "model_test", Type: model.EquipmentMilling, Manufacturer: "Test", Model: "CNC",
			MaxFeed: 5000,
		},
	}
}

func TestLifecycleProjectGETAndPOSTRequireCSRF(t *testing.T) {
	handler, repository := newLifecycleTestHandler(t)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/projects", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Создать проект") ||
		strings.Contains(response.Body.String(), "Проекты PostgreSQL") {
		t.Fatalf("GET /projects status = %d, body: %s", response.Code, response.Body.String())
	}

	token, cookie := cabinetCSRF(t, handler, "/projects/new")
	form := url.Values{"name": {"Demo lifecycle project"}, "description": {"test"}}

	request := httptest.NewRequest(http.MethodPost, "/projects/create", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF status = %d, want %d", response.Code, http.StatusForbidden)
	}

	form.Set("csrf_token", token)
	request = httptest.NewRequest(http.MethodPost, "/projects/create", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST with CSRF status = %d, body: %s", response.Code, response.Body.String())
	}
	if repository.project.Name != "Demo lifecycle project" || repository.project.CompanyID != storage.DemoCompanyID {
		t.Fatalf("created project = %#v", repository.project)
	}
	editPage := httptest.NewRecorder()
	handler.ServeHTTP(editPage, httptest.NewRequest(http.MethodGet, "/projects/"+repository.project.ID+"/edit", nil))
	if editPage.Code != http.StatusOK || !strings.Contains(editPage.Body.String(), "Demo lifecycle project") {
		t.Fatalf("GET project edit status = %d, body: %s", editPage.Code, editPage.Body.String())
	}
	form.Set("csrf_token", token)
	form.Set("name", "Updated lifecycle project")
	updateRequest := httptest.NewRequest(http.MethodPost, "/projects/"+repository.project.ID+"/save", strings.NewReader(form.Encode()))
	updateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateRequest.AddCookie(cookie)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusSeeOther || repository.project.Name != "Updated lifecycle project" {
		t.Fatalf("project update status = %d, record = %#v", updateResponse.Code, repository.project)
	}
}

func TestLifecycleHandlerDoesNotRequireJSONStore(t *testing.T) {
	repository := &lifecycleTestRepository{}
	company := model.Company{ID: "postgres-company", Name: "PostgreSQL company"}
	service := application.NewService(repository, repository)
	handler, err := NewLifecycleCabinetHandler(nil, company, service)
	if err != nil {
		t.Fatalf("create PostgreSQL-only handler: %v", err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/projects", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), company.Name) {
		t.Fatalf("GET /projects without JSON store status = %d, body: %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/projects" {
		t.Fatalf("GET / without JSON store status = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("")))
	if response.Code != http.StatusNotFound {
		t.Fatalf("POST / without JSON store status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestLifecyclePartUploadStoresOriginalAndHash(t *testing.T) {
	handler, repository := newLifecycleTestHandler(t)
	repository.project = application.ProjectRecord{
		ID: "project_test", CompanyID: storage.DemoCompanyID, Name: "Demo",
	}
	token, cookie := cabinetCSRF(t, handler, "/projects/new")
	source := []byte{0, 0xff, 'S', 'T', 'P'}
	var body strings.Builder
	writer := multipartWriter(t, &body)
	if err := writer.WriteField("csrf_token", token); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("part_name", "Square"); err != nil {
		t.Fatal(err)
	}
	partFile, err := writer.CreateFormFile("part_file", "square.stp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := partFile.Write(source); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/projects/"+repository.project.ID+"/parts/upload", strings.NewReader(body.String()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/revisions/") {
		t.Fatalf("upload status = %d, location %q, body: %s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	sum := sha256.Sum256(source)
	if repository.revision.SourceHash != hex.EncodeToString(sum[:]) {
		t.Fatal("repository did not receive the original source SHA-256")
	}
	if repository.revision.RevisionNumber != 1 || repository.revision.SourceFilename != "square.stp" || len(repository.revision.ParsedPart.Features) != 0 {
		t.Fatalf("attachment revision = %#v", repository.revision)
	}
	sourceResponse := httptest.NewRecorder()
	handler.ServeHTTP(sourceResponse, httptest.NewRequest(http.MethodGet, "/revisions/"+repository.revision.ID+"/source", nil))
	if sourceResponse.Code != http.StatusOK || sourceResponse.Body.String() != string(source) {
		t.Fatal("download endpoint did not return the original uploaded source")
	}
	revisionPage := httptest.NewRecorder()
	handler.ServeHTTP(revisionPage, httptest.NewRequest(http.MethodGet, "/revisions/"+repository.revision.ID, nil))
	if revisionPage.Code != http.StatusOK ||
		!strings.Contains(revisionPage.Body.String(), "Создать рабочую смету") ||
		!strings.Contains(revisionPage.Body.String(), "name=\"order_id\"") {
		t.Fatalf("GET revision estimate form status = %d, body: %s", revisionPage.Code, revisionPage.Body.String())
	}

	form := url.Values{
		"csrf_token":         {token},
		"estimate_name":      {"Smoke estimate"},
		"equipment_id":       {"equipment_test"},
		"markup_percent":     {"50"},
		"machine_time_hours": {"19.8"},
	}
	calculateRequest := httptest.NewRequest(http.MethodPost, "/revisions/"+repository.revision.ID+"/calculate", strings.NewReader(form.Encode()))
	calculateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	calculateRequest.AddCookie(cookie)
	calculateResponse := httptest.NewRecorder()
	handler.ServeHTTP(calculateResponse, calculateRequest)
	if calculateResponse.Code != http.StatusSeeOther || !strings.Contains(calculateResponse.Header().Get("Location"), "/estimates/estimate_test/versions/version_test") {
		t.Fatalf("calculate status = %d, location %q, body: %s", calculateResponse.Code, calculateResponse.Header().Get("Location"), calculateResponse.Body.String())
	}
	if len(repository.version.InputSnapshot) == 0 || len(repository.version.SettingsSnapshot) == 0 || len(repository.version.ResultSnapshot) == 0 {
		t.Fatal("calculation did not persist input, settings, and result snapshots")
	}
	if got := repository.version.MachineTimeHours; got < 0.329999 || got > 0.330001 {
		t.Fatalf("saved machine hours = %v, want 0.33", got)
	}

	form.Set("name", "Estimate from catalog")
	form.Set("quantity", "20")
	form.Set("global_material_id", "catalog-material")
	form.Set("material_weight_kg", "2.5")
	form.Set("global_tool_id", "catalog-tool")
	form.Set("tool_usage_minutes", "10")
	form.Set("tool_usage_reason", "CAM")
	form.Set("machine_time_reason", "Fusion")
	form.Set("fixed_operator_cost", "0")
	form.Set("fixed_setup_cost", "1000")
	form.Set("fixed_programming_cost", "1250")
	post := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	draft := post("/revisions/" + repository.revision.ID + "/estimates")
	if draft.Code != http.StatusSeeOther {
		t.Fatalf("create draft status = %d, body: %s", draft.Code, draft.Body.String())
	}
	var input application.EstimateInput
	if err := json.Unmarshal(repository.estimate.WorkingInput, &input); err != nil {
		t.Fatal(err)
	}
	if input.GlobalToolID != "catalog-tool" || input.GlobalMaterialID != "catalog-material" ||
		input.MaterialWeightKg != 2.5 || input.MachineTime == nil || input.MachineTime.Reason != "Fusion" {
		t.Fatalf("saved draft input = %#v", input)
	}
	if input.ProjectLabor == nil || input.ProjectLabor.Operator == nil || *input.ProjectLabor.Operator != 0 ||
		input.ProjectLabor.Setup == nil || *input.ProjectLabor.Setup != 1000 ||
		input.ProjectLabor.Programming == nil || *input.ProjectLabor.Programming != 1250 {
		t.Fatalf("saved draft fixed fees = %#v", input.ProjectLabor)
	}
	calculated := post("/estimates/" + repository.estimate.ID + "/calculate")
	if calculated.Code != http.StatusSeeOther {
		t.Fatalf("calculate draft status = %d, body: %s", calculated.Code, calculated.Body.String())
	}
	result, err := decodeEstimateResult(repository.version.ResultSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.CostBreakdown == nil || result.CostBreakdown.MaterialCost != 10000 || result.CostBreakdown.ToolingCost != 2000 {
		t.Fatalf("catalog material and tooling costs = %#v", result.CostBreakdown)
	}

	form.Del("global_tool_id")
	form.Set("consumable_id", "stock-tool")
	form.Set("tool_usage_minutes", "30")
	for _, test := range []struct {
		quantity, wear, purchase float64
	}{{1, 250, 0}, {2, 500, 1}} {
		form.Set("quantity", formatNumber(test.quantity))
		calculated := post("/estimates/" + repository.estimate.ID + "/calculate")
		if calculated.Code != http.StatusSeeOther {
			t.Fatalf("calculate stock tool: %d, %s", calculated.Code, calculated.Body.String())
		}
		result, err := decodeEstimateResult(repository.version.ResultSnapshot)
		if err != nil {
			t.Fatal(err)
		}
		cost := result.CostBreakdown
		if cost == nil || math.Abs(cost.ToolingCost-test.wear) > 1e-9 || cost.ToolSupply == nil ||
			!cost.ToolSupply.StockKnown || cost.ToolSupply.AvailableMinutes != 50 ||
			cost.ToolSupply.RequiredMinutes != 30*test.quantity ||
			cost.ToolSupply.PurchaseQuantity != test.purchase || cost.ToolSupply.PurchaseCost != 1000*test.purchase {
			t.Fatalf("quantity %v: unexpected stock tool breakdown: %+v", test.quantity, cost)
		}
		if cost.OperatorCost != 0 || cost.SetupCost != 1000 || cost.ProgrammingCost != 1250 || cost.BatchCost != 2250 {
			t.Fatalf("quantity %v: fixed fees should be charged once per batch: %+v", test.quantity, cost)
		}
		for _, warning := range cost.Warnings {
			if strings.Contains(warning, "Operator rate") {
				t.Fatalf("explicit zero operator fee should override the hourly rate: %q", warning)
			}
		}
	}

	form.Set("estimate_modules_present", "1")
	form.Set("estimate_module", application.ModuleOperator)
	form.Del("fixed_operator_cost")
	for _, wantOperator := range []float64{330, 0} {
		calculated := post("/estimates/" + repository.estimate.ID + "/calculate")
		if calculated.Code != http.StatusSeeOther {
			t.Fatalf("calculate modular estimate: %d, %s", calculated.Code, calculated.Body.String())
		}
		result, err := decodeEstimateResult(repository.version.ResultSnapshot)
		if err != nil {
			t.Fatal(err)
		}
		cost := result.CostBreakdown
		if cost == nil || math.Abs(cost.OperatorCost-wantOperator) > 1e-9 ||
			cost.ToolingCost != 0 || cost.MaterialCost != 0 || cost.SetupCost != 0 || cost.ProgrammingCost != 0 {
			t.Fatalf("removed modules must not affect calculation: %+v", cost)
		}
		var saved application.EstimateInput
		if err := json.Unmarshal(repository.estimate.WorkingInput, &saved); err != nil ||
			saved.Modules == nil || saved.Modules.Has(application.ModuleOperator) != (wantOperator > 0) {
			t.Fatalf("module selection was not saved: %#v, err=%v", saved.Modules, err)
		}
		form.Del("estimate_module")
	}
}

func newLifecycleTestHandler(t *testing.T) (http.Handler, *lifecycleTestRepository) {
	t.Helper()
	store, err := storage.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create JSON store: %v", err)
	}
	repository := &lifecycleTestRepository{}
	service := application.NewService(repository, repository)
	company, err := service.GetCompany(context.Background(), storage.DemoCompanyID)
	if err != nil {
		t.Fatalf("get lifecycle company: %v", err)
	}
	handler, err := NewLifecycleCabinetHandler(store, company, service)
	if err != nil {
		t.Fatalf("create lifecycle handler: %v", err)
	}
	return handler, repository
}

func multipartWriter(t *testing.T, body *strings.Builder) *multipart.Writer {
	t.Helper()
	return multipart.NewWriter(body)
}

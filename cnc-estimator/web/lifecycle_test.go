package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smetchic/application"
	"smetchic/model"
	"smetchic/storage"
)

type lifecycleTestRepository struct {
	application.Repository
	projects []application.ProjectRecord
	project  application.ProjectRecord
	part     application.PartRecord
	revision application.PartRevisionRecord
	estimate application.EstimateRecord
	version  application.EstimateVersionRecord
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
	estimate.ID = "estimate_test"
	estimate.CreatedAt = time.Now()
	version.EstimateID = estimate.ID
	version.ID = "version_test"
	version.VersionNumber = 1
	repository.estimate, repository.version = estimate, version
	return estimate, version, nil
}

func (repository *lifecycleTestRepository) GetEstimate(_ context.Context, companyID, estimateID string) (application.EstimateRecord, error) {
	if repository.estimate.ID != estimateID || repository.estimate.CompanyID != companyID {
		return application.EstimateRecord{}, application.ErrNotFound
	}
	return repository.estimate, nil
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
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Проекты PostgreSQL") {
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
	source, err := os.ReadFile(filepath.Join("..", "examples", "square.part"))
	if err != nil {
		t.Fatalf("read example part: %v", err)
	}
	var body strings.Builder
	writer := multipartWriter(t, &body)
	if err := writer.WriteField("csrf_token", token); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("part_name", "Square"); err != nil {
		t.Fatal(err)
	}
	partFile, err := writer.CreateFormFile("part_file", "square.part")
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
	if repository.revision.SourceContent != string(source) || repository.revision.SourceHash != hex.EncodeToString(sum[:]) {
		t.Fatal("repository did not receive original source and its SHA-256")
	}
	if repository.revision.RevisionNumber != 1 || repository.revision.ParsedPart.SourceFile != "square.part" {
		t.Fatalf("parsed revision = %#v", repository.revision)
	}
	sourceResponse := httptest.NewRecorder()
	handler.ServeHTTP(sourceResponse, httptest.NewRequest(http.MethodGet, "/revisions/"+repository.revision.ID+"/source", nil))
	if sourceResponse.Code != http.StatusOK || sourceResponse.Body.String() != string(source) {
		t.Fatal("download endpoint did not return the original uploaded source")
	}

	form := url.Values{
		"csrf_token":     {token},
		"estimate_name":  {"Smoke estimate"},
		"equipment_id":   {"equipment_test"},
		"markup_percent": {"50"},
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

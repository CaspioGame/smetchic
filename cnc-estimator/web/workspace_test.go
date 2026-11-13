package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smetchic/application"
	"smetchic/model"
)

type workspaceTestRepository struct {
	application.Repository
	*webAuthRepository
}

func (repository *workspaceTestRepository) GetCompany(_ context.Context, companyID string) (model.Company, error) {
	return model.Company{ID: companyID, Name: "Workspace test company"}, nil
}

func (repository *workspaceTestRepository) GetCompanySettings(_ context.Context, companyID string) (model.CompanySettings, error) {
	return model.CompanySettings{CompanyID: companyID, Currency: "RUB", DefaultMarkupPercent: 50}, nil
}

func (repository *workspaceTestRepository) GetEquipment(_ context.Context, companyID, equipmentID string) (model.Equipment, error) {
	return model.Equipment{}, application.ErrNotFound
}

func (repository *workspaceTestRepository) ListEquipment(context.Context, string) ([]model.Equipment, error) {
	return []model.Equipment{{ID: "machine-test", Name: "Фрезерный станок для теста"}}, nil
}

func (repository *workspaceTestRepository) ListCompanyUsers(context.Context, string) ([]application.CompanyUserRecord, error) {
	return nil, nil
}

func (repository *workspaceTestRepository) ListEquipmentModels(context.Context) ([]model.EquipmentModel, error) {
	return nil, nil
}

func (repository *workspaceTestRepository) ListMaterials(context.Context, string) ([]model.Material, error) {
	return nil, nil
}

func (repository *workspaceTestRepository) ListGlobalMaterials(context.Context) ([]model.GlobalMaterial, error) {
	return []model.GlobalMaterial{
		{
			ID: "aluminum-6061", Name: "Aluminum", Grade: "6061",
			Category: model.MaterialCategoryAluminum, Density: 2700,
			ReferencePriceKg: 500, Currency: "RUB",
			Description: "General-purpose aluminum alloy.",
		},
		{
			ID: "steel-45", Name: "Steel", Grade: "45",
			Category: model.MaterialCategorySteel, Density: 7850,
			ReferencePriceKg: 180, Currency: "RUB",
			Description: "Carbon steel.",
		},
	}, nil
}

func (repository *workspaceTestRepository) GetCompanyEconomics(_ context.Context, companyID string) (model.CompanyEconomics, error) {
	return model.CompanyEconomics{
		CompanyID: companyID, DefaultMarkupPercent: 50, Currency: "RUB",
		RentAllocationMethod: model.AllocationByProductionHours,
	}, nil
}

func (*workspaceTestRepository) SaveCompanyEconomics(context.Context, model.CompanyEconomics) error {
	return nil
}

func (*workspaceTestRepository) ListEquipmentEconomics(context.Context, string) ([]model.EquipmentEconomics, error) {
	return []model.EquipmentEconomics{{EquipmentID: "machine-test"}}, nil
}

func (*workspaceTestRepository) GetEquipmentEconomics(context.Context, string, string) (*model.EquipmentEconomics, error) {
	return nil, nil
}

func (*workspaceTestRepository) SaveEquipmentEconomics(context.Context, model.EquipmentEconomics) error {
	return nil
}

func (*workspaceTestRepository) DeleteEquipmentEconomics(context.Context, string, string) error {
	return nil
}

func (*workspaceTestRepository) ListPersonnelRates(context.Context, string) ([]model.PersonnelRate, error) {
	return nil, nil
}

func (*workspaceTestRepository) SavePersonnelRate(context.Context, model.PersonnelRate) error {
	return nil
}

func (*workspaceTestRepository) DeletePersonnelRate(context.Context, string, string) error {
	return nil
}

func (*workspaceTestRepository) ListCompanyOverheads(context.Context, string) ([]model.CompanyOverhead, error) {
	return nil, nil
}

func (*workspaceTestRepository) SaveCompanyOverhead(context.Context, model.CompanyOverhead) error {
	return nil
}

func (*workspaceTestRepository) DeleteCompanyOverhead(context.Context, string, string) error {
	return nil
}

func (*workspaceTestRepository) ListStockBlanks(context.Context, string) ([]model.StockBlank, error) {
	return nil, nil
}

func (*workspaceTestRepository) SaveStockBlank(context.Context, model.StockBlank) error {
	return nil
}

func (*workspaceTestRepository) DeleteStockBlank(context.Context, string, string) error {
	return nil
}

func (repository *workspaceTestRepository) ListConsumables(context.Context, string) ([]model.Consumable, error) {
	return nil, nil
}

func (repository *workspaceTestRepository) ListOrders(context.Context, string) ([]application.OrderRecord, error) {
	return nil, nil
}

func (repository *workspaceTestRepository) ListProjects(context.Context, string) ([]application.ProjectRecord, error) {
	return nil, nil
}

func TestAuthenticatedWorkspacePagesRenderWithoutJSONStore(t *testing.T) {
	repository := &workspaceTestRepository{webAuthRepository: newWebAuthRepository()}
	user := model.User{ID: "workspace-user", Email: "owner@example.test", Name: "Workspace owner"}
	repository.users[user.ID] = application.AuthUserRecord{User: user, Active: true}
	repository.memberships[user.ID] = []application.AuthMembership{{
		Company: model.Company{ID: "workspace-company", Name: "Workspace test company"},
		Role:    model.CompanyRoleOwner,
	}}
	lifecycle := application.NewService(repository, repository)
	handler, err := NewAuthenticatedLifecycleCabinetHandler(
		nil, lifecycle, application.NewAuthenticationService(repository),
	)
	if err != nil {
		t.Fatal(err)
	}
	server := handler.(*Server)
	session, err := server.sessions.create(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"/company", "/equipment", "/equipment/new", "/materials", "/materials/new",
		"/consumables", "/consumables/new", "/catalog/equipment", "/catalog/materials",
		"/catalog/materials/aluminum-6061", "/orders", "/orders/new", "/projects",
		"/economics", "/economics?equipment_id=machine-test", "/stock-blanks", "/stock-blanks/new",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: session.CSRFToken})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, body: %s", path, response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "Workspace owner") {
				t.Fatalf("GET %s did not render the authenticated workspace", path)
			}
			if path == "/economics" && !strings.Contains(response.Body.String(), "<td>Фрезерный станок для теста</td>") {
				t.Fatal("equipment economics table did not show the equipment name")
			}
		})
	}
}

func TestGlobalMaterialsAreSearchableAndReadOnly(t *testing.T) {
	repository := &workspaceTestRepository{webAuthRepository: newWebAuthRepository()}
	user := model.User{ID: "workspace-operator", Email: "operator@example.test", Name: "Operator"}
	repository.users[user.ID] = application.AuthUserRecord{User: user, Active: true}
	repository.memberships[user.ID] = []application.AuthMembership{{
		Company: model.Company{ID: "workspace-company", Name: "Workspace test company"},
		Role:    model.CompanyRoleOperator,
	}}
	lifecycle := application.NewService(repository, repository)
	handler, err := NewAuthenticatedLifecycleCabinetHandler(
		nil, lifecycle, application.NewAuthenticationService(repository),
	)
	if err != nil {
		t.Fatal(err)
	}
	server := handler.(*Server)
	session, err := server.sessions.create(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/catalog/materials?q=6061", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: session.CSRFToken})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Aluminum") ||
		strings.Contains(response.Body.String(), "Steel") ||
		!strings.Contains(response.Body.String(), "500.00 RUB/кг") {
		t.Fatalf("global material search status = %d, body: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/catalog/materials/save", strings.NewReader(""))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("global material write status = %d, want 405", response.Code)
	}
}

func TestWorkspaceRoleRestrictions(t *testing.T) {
	repository := &workspaceTestRepository{webAuthRepository: newWebAuthRepository()}
	user := model.User{ID: "workspace-manager", Email: "manager@example.test", Name: "Manager"}
	repository.users[user.ID] = application.AuthUserRecord{User: user, Active: true}
	repository.memberships[user.ID] = []application.AuthMembership{{
		Company: model.Company{ID: "workspace-company", Name: "Workspace test company"},
		Role:    model.CompanyRoleManager,
	}}
	lifecycle := application.NewService(repository, repository)
	handler, err := NewAuthenticatedLifecycleCabinetHandler(
		nil, lifecycle, application.NewAuthenticationService(repository),
	)
	if err != nil {
		t.Fatal(err)
	}
	server := handler.(*Server)
	session, err := server.sessions.create(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/company/save", strings.NewReader(""))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden ||
		!strings.Contains(response.Body.String(), "Недостаточно прав") {
		t.Fatalf("manager company update status = %d, body: %s", response.Code, response.Body.String())
	}
}

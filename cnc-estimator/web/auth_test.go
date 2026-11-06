package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"smetchic/application"
	"smetchic/model"
)

type webAuthRepository struct {
	mu          sync.Mutex
	users       map[string]application.AuthUserRecord
	memberships map[string][]application.AuthMembership
}

func newWebAuthRepository() *webAuthRepository {
	return &webAuthRepository{
		users:       make(map[string]application.AuthUserRecord),
		memberships: make(map[string][]application.AuthMembership),
	}
}

func (repository *webAuthRepository) GetAuthUserByEmail(_ context.Context, email string) (application.AuthUserRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, record := range repository.users {
		if strings.EqualFold(record.User.Email, email) {
			return record, nil
		}
	}
	return application.AuthUserRecord{}, application.ErrNotFound
}

func (repository *webAuthRepository) GetAuthUserByID(_ context.Context, userID string) (application.AuthUserRecord, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	record, ok := repository.users[userID]
	if !ok {
		return application.AuthUserRecord{}, application.ErrNotFound
	}
	return record, nil
}

func (repository *webAuthRepository) ListUserMemberships(_ context.Context, userID string) ([]application.AuthMembership, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return append([]application.AuthMembership(nil), repository.memberships[userID]...), nil
}

func (repository *webAuthRepository) CreateUserWithMembership(
	_ context.Context, user model.User, passwordHash, companyID string, role model.CompanyRole,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, record := range repository.users {
		if strings.EqualFold(record.User.Email, user.Email) {
			return errors.New("duplicate email")
		}
	}
	record := application.AuthUserRecord{User: user, PasswordHash: passwordHash, Active: true}
	repository.users[user.ID] = record
	repository.memberships[user.ID] = []application.AuthMembership{{
		Company: model.Company{ID: companyID, Name: "Authenticated company"}, Role: role,
	}}
	return nil
}

func TestLoginSessionLogoutAndProtectedRoutes(t *testing.T) {
	handler, repository := newAuthenticatedTestHandler(t)
	user, err := application.NewAuthenticationService(repository).CreateUser(
		context.Background(), "owner@example.test", "Test owner", "correct-password-test",
		"company-a", model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatal(err)
	}

	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/login", nil))
	if loginPage.Code != http.StatusOK || !strings.Contains(loginPage.Body.String(), "Вход в CNC Estimator") {
		t.Fatalf("GET /login status = %d, body: %s", loginPage.Code, loginPage.Body.String())
	}
	loginSession, loginCSRF := authCookies(t, loginPage)
	csrf := csrfFromBody(t, loginPage.Body.String())

	wrong := postAuthForm(handler, "/login", url.Values{
		"email": {"owner@example.test"}, "password": {"wrong-password-test"}, "csrf_token": {csrf},
	}, loginSession, loginCSRF)
	if wrong.Code != http.StatusUnauthorized || !strings.Contains(wrong.Body.String(), "Неверный email или пароль") {
		t.Fatalf("invalid login status = %d, body: %s", wrong.Code, wrong.Body.String())
	}

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/projects", nil))
	if anonymous.Code != http.StatusSeeOther || anonymous.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous protected route status = %d, location %q", anonymous.Code, anonymous.Header().Get("Location"))
	}

	login := postAuthForm(handler, "/login", url.Values{
		"email": {"owner@example.test"}, "password": {"correct-password-test"}, "csrf_token": {csrf},
	}, loginSession, loginCSRF)
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/projects" {
		t.Fatalf("valid login status = %d, location %q, body: %s", login.Code, login.Header().Get("Location"), login.Body.String())
	}
	sessionCookie, csrfCookie := authCookies(t, login)
	if sessionCookie.Value == loginSession.Value {
		t.Fatal("login did not rotate the pre-authentication session ID")
	}
	if strings.Contains(sessionCookie.Value, user.ID) || strings.Contains(sessionCookie.Value, "correct-password-test") {
		t.Fatal("session cookie contains user credentials instead of an opaque identifier")
	}
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie security attributes = %#v", sessionCookie)
	}

	protected := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects", nil)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
	handler.ServeHTTP(protected, request)
	if protected.Code != http.StatusOK || !strings.Contains(protected.Body.String(), "Authenticated company") {
		t.Fatalf("authenticated route status = %d, body: %s", protected.Code, protected.Body.String())
	}

	logout := postAuthForm(handler, "/logout", url.Values{"csrf_token": {csrfCookie.Value}}, sessionCookie, csrfCookie)
	if logout.Code != http.StatusSeeOther || logout.Header().Get("Location") != "/login" {
		t.Fatalf("logout status = %d, location %q", logout.Code, logout.Header().Get("Location"))
	}
	afterLogout := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/projects", nil)
	request.AddCookie(sessionCookie)
	handler.ServeHTTP(afterLogout, request)
	if afterLogout.Code != http.StatusSeeOther || afterLogout.Header().Get("Location") != "/login" {
		t.Fatalf("request after logout status = %d, location %q", afterLogout.Code, afterLogout.Header().Get("Location"))
	}
	if _, ok := repository.users[user.ID]; !ok {
		t.Fatal("login unexpectedly changed the persisted user")
	}
}

func TestDeactivatedUserSessionIsRejected(t *testing.T) {
	handler, repository := newAuthenticatedTestHandler(t)
	user, err := application.NewAuthenticationService(repository).CreateUser(
		context.Background(), "deactivate@example.test", "Deactivated", "correct-password-test",
		"company-a", model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/login", nil))
	preAuthSession, preAuthCSRF := authCookies(t, loginPage)
	login := postAuthForm(handler, "/login", url.Values{
		"email": {"deactivate@example.test"}, "password": {"correct-password-test"},
		"csrf_token": {csrfFromBody(t, loginPage.Body.String())},
	}, preAuthSession, preAuthCSRF)
	sessionCookie, csrfCookie := authCookies(t, login)
	repository.mu.Lock()
	record := repository.users[user.ID]
	record.Active = false
	repository.users[user.ID] = record
	repository.mu.Unlock()

	request := httptest.NewRequest(http.MethodGet, "/projects", nil)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("inactive session status = %d, location %q", response.Code, response.Header().Get("Location"))
	}
}

func TestLoginRequiresCSRFAndGenericUnknownEmailError(t *testing.T) {
	handler, repository := newAuthenticatedTestHandler(t)
	_, err := application.NewAuthenticationService(repository).CreateUser(
		context.Background(), "known@example.test", "Known", "correct-password-test",
		"company-a", model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/login", nil))
	sessionCookie, csrfCookie := authCookies(t, loginPage)
	token := csrfFromBody(t, loginPage.Body.String())

	withoutCSRF := postAuthForm(handler, "/login", url.Values{
		"email": {"known@example.test"}, "password": {"correct-password-test"},
	}, sessionCookie, csrfCookie)
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("login without CSRF status = %d", withoutCSRF.Code)
	}
	unknown := postAuthForm(handler, "/login", url.Values{
		"email": {"unknown@example.test"}, "password": {"wrong-password-test"}, "csrf_token": {token},
	}, sessionCookie, csrfCookie)
	if unknown.Code != http.StatusUnauthorized || !strings.Contains(unknown.Body.String(), "Неверный email или пароль") {
		t.Fatalf("unknown login status = %d, body: %s", unknown.Code, unknown.Body.String())
	}
}

func TestMultipleMembershipsAreHandledExplicitly(t *testing.T) {
	handler, repository := newAuthenticatedTestHandler(t)
	service := application.NewAuthenticationService(repository)
	user, err := service.CreateUser(context.Background(), "multi@example.test", "Multiple", "correct-password-test", "company-a", model.CompanyRoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	repository.mu.Lock()
	repository.memberships[user.ID] = append(repository.memberships[user.ID], application.AuthMembership{
		Company: model.Company{ID: "company-b", Name: "Other company"}, Role: model.CompanyRoleOperator,
	})
	repository.mu.Unlock()

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/login", nil))
	sessionCookie, csrfCookie := authCookies(t, page)
	login := postAuthForm(handler, "/login", url.Values{
		"email": {"multi@example.test"}, "password": {"correct-password-test"},
		"csrf_token": {csrfFromBody(t, page.Body.String())},
	}, sessionCookie, csrfCookie)
	sessionCookie, csrfCookie = authCookies(t, login)
	request := httptest.NewRequest(http.MethodGet, "/projects", nil)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusForbidden || !strings.Contains(result.Body.String(), "ровно одна компания") {
		t.Fatalf("multiple memberships status = %d, body: %s", result.Code, result.Body.String())
	}
	logout := postAuthForm(handler, "/logout", url.Values{"csrf_token": {csrfCookie.Value}}, sessionCookie, csrfCookie)
	if logout.Code != http.StatusSeeOther {
		t.Fatalf("logout for multi-company user status = %d", logout.Code)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	store := newSessionStore()
	session, err := store.create("user-1")
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	session.ExpiresAt = time.Now().Add(-time.Minute)
	store.sessions[session.ID] = session
	store.mu.Unlock()
	if _, ok := store.get(session.ID); ok {
		t.Fatal("expired session was accepted")
	}
}

func TestAuthenticatedCompanyScopesLegacyAndLifecycleIDs(t *testing.T) {
	authRepository := newWebAuthRepository()
	lifecycleRepository := &lifecycleTestRepository{
		project: application.ProjectRecord{
			ID: "project-b", CompanyID: "company-b", Name: "Private project B",
		},
	}
	service := application.NewService(lifecycleRepository, lifecycleRepository)
	handler, err := NewAuthenticatedLifecycleCabinetHandler(
		nil, service, application.NewAuthenticationService(authRepository),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.NewAuthenticationService(authRepository).CreateUser(
		context.Background(), "company-a@example.test", "Company A", "correct-password-test",
		"company-a", model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/login", nil))
	sessionCookie, csrfCookie := authCookies(t, loginPage)
	login := postAuthForm(handler, "/login", url.Values{
		"email": {"company-a@example.test"}, "password": {"correct-password-test"},
		"csrf_token": {csrfFromBody(t, loginPage.Body.String())},
	}, sessionCookie, csrfCookie)
	sessionCookie, csrfCookie = authCookies(t, login)

	request := httptest.NewRequest(http.MethodGet, "/projects/project-b/edit?company_id=company-b", nil)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "Private project B") {
		t.Fatalf("cross-company project response status = %d, body: %s", response.Code, response.Body.String())
	}
}

func newAuthenticatedTestHandler(t *testing.T) (http.Handler, *webAuthRepository) {
	t.Helper()
	authRepository := newWebAuthRepository()
	lifecycleRepository := &lifecycleTestRepository{}
	service := application.NewService(lifecycleRepository, lifecycleRepository)
	handler, err := NewAuthenticatedLifecycleCabinetHandler(
		nil, service, application.NewAuthenticationService(authRepository),
	)
	if err != nil {
		t.Fatalf("create authenticated handler: %v", err)
	}
	return handler, authRepository
}

func authCookies(t *testing.T, response *httptest.ResponseRecorder) (*http.Cookie, *http.Cookie) {
	t.Helper()
	var session, csrf *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		switch cookie.Name {
		case sessionCookieName:
			session = cookie
		case csrfCookieName:
			csrf = cookie
		}
	}
	if session == nil || csrf == nil {
		t.Fatalf("session cookies missing: %#v", response.Result().Cookies())
	}
	return session, csrf
}

func csrfFromBody(t *testing.T, body string) string {
	t.Helper()
	match := csrfInputPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("CSRF input not found: %s", body)
	}
	return match[1]
}

func postAuthForm(handler http.Handler, path string, form url.Values, session, csrf *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if session != nil {
		request.AddCookie(session)
	}
	if csrf != nil {
		request.AddCookie(csrf)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

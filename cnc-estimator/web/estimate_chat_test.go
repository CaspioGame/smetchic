package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"smetchic/application"
	"smetchic/model"
)

type estimateChatTestRepository struct {
	*workspaceTestRepository
	application.EstimateDocumentRepository
	chats    map[string]application.ChatDialog
	projects map[string]application.ProjectRecord
}

func TestChatGroundsFixedPaymentWithoutChangingTime(t *testing.T) {
	reply := estimateAssistantReply{Action: &chatAction{Operation: "update_estimate"}, Changes: map[string]json.Number{
		"setup_time_minutes": "1000", "machine_time_hours": "19.8",
	}}
	groundChatReferences(&reply, "Установи время обработки 19,8 минуты на деталь и наладку фиксированно 1000 рублей за партию.", chatCatalog{})
	if reply.Changes["fixed_setup_cost"] != "1000" || reply.Changes["setup_time_minutes"] != "" || reply.Changes["machine_time_hours"] != "19.8" {
		t.Fatalf("fixed payment confused with time: %v", reply.Changes)
	}
}

func (r *estimateChatTestRepository) CreateChat(_ context.Context, c application.ChatDialog) (application.ChatDialog, error) {
	r.chats[c.ID] = c
	return c, nil
}
func (r *estimateChatTestRepository) ListChats(_ context.Context, company, user string) ([]application.ChatDialog, error) {
	var out []application.ChatDialog
	for _, c := range r.chats {
		if c.CompanyID == company && c.UserID == user {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *estimateChatTestRepository) GetChat(_ context.Context, company, user, id string) (application.ChatDialog, error) {
	c, ok := r.chats[id]
	if !ok || c.CompanyID != company || c.UserID != user {
		return application.ChatDialog{}, application.ErrNotFound
	}
	return c, nil
}
func (r *estimateChatTestRepository) ClaimChat(ctx context.Context, company, user, id string, rev int) error {
	c, err := r.GetChat(ctx, company, user, id)
	if err != nil || c.Revision != rev {
		return application.ErrNotFound
	}
	c.Revision++
	r.chats[id] = c
	return nil
}
func (r *estimateChatTestRepository) SaveChat(_ context.Context, c application.ChatDialog) error {
	r.chats[c.ID] = c
	return nil
}
func (*estimateChatTestRepository) CreateChatEstimate(context.Context, application.EstimateRecord, application.PartRecord, application.PartRevisionRecord) (application.EstimateRecord, error) {
	panic("unexpected create estimate")
}
func (*estimateChatTestRepository) ListCompanyEstimates(context.Context, string) ([]application.EstimateRecord, error) {
	return nil, nil
}
func (r *estimateChatTestRepository) CreateProject(_ context.Context, p application.ProjectRecord) error {
	r.projects[p.ID] = p
	return nil
}
func (r *estimateChatTestRepository) GetProject(_ context.Context, company, id string) (application.ProjectRecord, error) {
	p, ok := r.projects[id]
	if !ok || p.CompanyID != company {
		return p, application.ErrNotFound
	}
	return p, nil
}
func (r *estimateChatTestRepository) ListProjects(_ context.Context, company string) ([]application.ProjectRecord, error) {
	var out []application.ProjectRecord
	for _, p := range r.projects {
		if p.CompanyID == company {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestChatStartsEmptyPersistsAndCreatesProjectWithoutNavigation(t *testing.T) {
	repo := &estimateChatTestRepository{workspaceTestRepository: &workspaceTestRepository{webAuthRepository: newWebAuthRepository()}, chats: map[string]application.ChatDialog{}, projects: map[string]application.ProjectRecord{}}
	for _, id := range []string{"user", "other-user"} {
		user := model.User{ID: id, Name: id, Email: id + "@example.test"}
		repo.users[id] = application.AuthUserRecord{User: user, Active: true}
		repo.memberships[id] = []application.AuthMembership{{Company: model.Company{ID: "company", Name: "Компания"}, Role: model.CompanyRoleManager}}
	}
	newServer := func() *Server {
		handler, err := NewAuthenticatedLifecycleCabinetHandler(nil, application.NewService(repo, repo), application.NewAuthenticationService(repo))
		if err != nil {
			t.Fatal(err)
		}
		return handler.(*Server)
	}
	server := newServer()
	server.chatAssistant = func(_ context.Context, _ string, _ map[string]string, _ chatCatalog) (estimateAssistantReply, error) {
		return estimateAssistantReply{Reply: "Создать проект «Корпус»?", Action: &chatAction{Operation: "create_project", Name: "Корпус"}}, nil
	}
	session, err := server.sessions.create("user")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: session.CSRFToken})
		res := httptest.NewRecorder()
		server.ServeHTTP(res, req)
		return res
	}
	page := request(http.MethodGet, "/chat", nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "chat-message") || !strings.Contains(page.Body.String(), "Новый чат") {
		t.Fatalf("new chat page: %d %s", page.Code, page.Body.String())
	}
	form := url.Values{"csrf_token": {session.CSRFToken}, "assistant_prompt": {"Создай проект Корпус"}}
	posted := request(http.MethodPost, "/chat", form)
	if posted.Code != 303 || !strings.HasPrefix(posted.Header().Get("Location"), "/chat?dialog_id=") {
		t.Fatalf("send chat: %d %s", posted.Code, posted.Body.String())
	}
	if len(repo.projects) != 0 {
		t.Fatal("proposal executed before confirmation")
	}
	var chat application.ChatDialog
	for _, c := range repo.chats {
		chat = c
	}
	var state dialogState
	if err := json.Unmarshal(chat.State, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Turns) != 1 || !state.Turns[0].Pending {
		t.Fatal("pending action not saved")
	}
	confirm := url.Values{"csrf_token": {session.CSRFToken}, "dialog_id": {chat.ID}, "dialog_revision": {strconv.Itoa(chat.Revision)}, "confirm": {state.Turns[0].ID}}
	res := request(http.MethodPost, "/chat", confirm)
	if res.Code != 303 || len(repo.projects) != 1 {
		t.Fatalf("confirm project: %d %s", res.Code, res.Body.String())
	}
	duplicate := request(http.MethodPost, "/chat", confirm)
	if duplicate.Code != 409 || len(repo.projects) != 1 {
		t.Fatal("repeated confirmation duplicated project")
	}
	server = newServer()
	session, err = server.sessions.create("user")
	if err != nil {
		t.Fatal(err)
	}
	restored := request(http.MethodGet, "/chat?dialog_id="+chat.ID, nil)
	if restored.Code != 200 || !strings.Contains(restored.Body.String(), "Создан проект") {
		t.Fatalf("chat lost after restart: %d %s", restored.Code, restored.Body.String())
	}
	session, err = server.sessions.create("other-user")
	if err != nil {
		t.Fatal(err)
	}
	denied := request(http.MethodGet, "/chat?dialog_id="+chat.ID, nil)
	if denied.Code != 404 || strings.Contains(denied.Body.String(), "Создан проект") {
		t.Fatal("other user can read private chat")
	}
}

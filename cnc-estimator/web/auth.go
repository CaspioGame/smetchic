package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"smetchic/application"
	"smetchic/model"
)

const (
	sessionCookieName = "cnc_session"
	csrfCookieName    = "cnc_csrf"
)

type authContextKey struct{}

type requestIdentity struct {
	User    model.User
	Company model.Company
	Role    model.CompanyRole
	Session serverSession
}

type loginPage struct {
	pageBase
}

type registerPage struct {
	pageBase
	Email       string
	DisplayName string
	CompanyName string
}

func identityFromRequest(r *http.Request) (requestIdentity, bool) {
	identity, ok := r.Context().Value(authContextKey{}).(requestIdentity)
	return identity, ok
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if session, ok := s.sessionFromRequest(r); ok && session.UserID != "" {
			record, memberships, err := s.authentication.ResolveUser(r.Context(), session.UserID)
			if errors.Is(err, application.ErrInactiveUser) {
				s.sessions.delete(session.ID)
				s.clearSessionCookies(w, r)
			} else if errors.Is(err, application.ErrNotFound) {
				s.sessions.delete(session.ID)
				s.clearSessionCookies(w, r)
			} else if err != nil {
				log.Printf("resolve user on login page: %v", err)
				http.Error(w, "Не удалось проверить сессию. Попробуйте позже.", http.StatusInternalServerError)
				return
			} else if len(memberships) != 1 {
				s.renderMembershipError(w, r, session, record.User)
				return
			} else {
				http.Redirect(w, r, "/projects", http.StatusSeeOther)
				return
			}
		}
		session, err := s.ensurePreAuthSession(w, r)
		if err != nil {
			http.Error(w, "Не удалось подготовить вход. Попробуйте позже.", http.StatusInternalServerError)
			return
		}
		s.render(w, r, "login.html", http.StatusOK, loginPage{
			pageBase: pageBase{Title: "Вход", CSRF: session.CSRFToken, Error: r.URL.Query().Get("error")},
		})
	case http.MethodPost:
		s.login(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
	}
}

func (s *Server) serveRegister(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if session, ok := s.sessionFromRequest(r); ok && session.UserID != "" {
			http.Redirect(w, r, "/projects", http.StatusSeeOther)
			return
		}
		session, err := s.ensurePreAuthSession(w, r)
		if err != nil {
			http.Error(w, "Не удалось подготовить регистрацию. Попробуйте позже.", http.StatusInternalServerError)
			return
		}
		s.render(w, r, "register.html", http.StatusOK, registerPage{
			pageBase: pageBase{Title: "Регистрация", CSRF: session.CSRFToken},
		})
	case http.MethodPost:
		s.register(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
	}
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Некорректные данные формы.", http.StatusBadRequest)
		return
	}
	session, ok := s.sessionFromRequest(r)
	if !ok || session.UserID != "" || !s.validCSRF(r, session) {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу.", http.StatusForbidden)
		return
	}
	page := registerPage{
		pageBase:    pageBase{Title: "Регистрация", CSRF: session.CSRFToken},
		Email:       strings.TrimSpace(r.FormValue("email")),
		DisplayName: strings.TrimSpace(r.FormValue("name")),
		CompanyName: strings.TrimSpace(r.FormValue("company_name")),
	}
	password := r.FormValue("password")
	if password != r.FormValue("password_confirmation") {
		page.Error = "Пароли не совпадают."
		s.render(w, r, "register.html", http.StatusBadRequest, page)
		return
	}
	user, _, err := s.authentication.RegisterCompany(
		r.Context(), page.Email, page.DisplayName, password, page.CompanyName,
	)
	if errors.Is(err, application.ErrEmailAlreadyExists) {
		page.Error = "Этот email уже зарегистрирован."
		s.render(w, r, "register.html", http.StatusConflict, page)
		return
	}
	if err != nil {
		var validationError *application.ValidationError
		if errors.As(err, &validationError) {
			page.Error = validationError.Message
			s.render(w, r, "register.html", http.StatusBadRequest, page)
			return
		}
		log.Printf("register company and owner: %v", err)
		page.Error = "Не удалось создать аккаунт. Проверьте данные и повторите попытку."
		s.render(w, r, "register.html", http.StatusInternalServerError, page)
		return
	}
	s.sessions.delete(session.ID)
	authSession, err := s.sessions.create(user.ID)
	if err != nil {
		log.Printf("create registration session: %v", err)
		http.Error(w, "Аккаунт создан, но не удалось выполнить вход. Войдите через форму.", http.StatusInternalServerError)
		return
	}
	s.setSessionCookies(w, r, authSession)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Некорректные данные формы.", http.StatusBadRequest)
		return
	}
	session, ok := s.sessionFromRequest(r)
	if !ok || session.UserID != "" || !s.validCSRF(r, session) {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу.", http.StatusForbidden)
		return
	}
	user, err := s.authentication.Login(r.Context(), r.FormValue("email"), r.FormValue("password"))
	if errors.Is(err, application.ErrInvalidCredentials) {
		s.render(w, r, "login.html", http.StatusUnauthorized, loginPage{
			pageBase: pageBase{Title: "Вход", CSRF: session.CSRFToken, Error: "Неверный email или пароль."},
		})
		return
	}
	if err != nil {
		log.Printf("authenticate user: %v", err)
		http.Error(w, "Не удалось выполнить вход. Попробуйте позже.", http.StatusInternalServerError)
		return
	}

	s.sessions.delete(session.ID)
	authSession, err := s.sessions.create(user.ID)
	if err != nil {
		log.Printf("create authenticated session: %v", err)
		http.Error(w, "Не удалось выполнить вход. Попробуйте позже.", http.StatusInternalServerError)
		return
	}
	s.setSessionCookies(w, r, authSession)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := s.sessionFromRequest(r)
		if !ok || session.UserID == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, memberships, err := s.authentication.ResolveUser(r.Context(), session.UserID)
		if errors.Is(err, application.ErrInactiveUser) || errors.Is(err, application.ErrNotFound) {
			s.sessions.delete(session.ID)
			s.clearSessionCookies(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			log.Printf("resolve authenticated user: %v", err)
			http.Error(w, "Не удалось проверить доступ. Попробуйте позже.", http.StatusInternalServerError)
			return
		}
		identity := requestIdentity{
			User: user.User, Session: session,
		}
		if len(memberships) == 1 {
			identity.Company, identity.Role = memberships[0].Company, memberships[0].Role
		}
		ctx := context.WithValue(r.Context(), authContextKey{}, identity)
		authenticatedRequest := r.WithContext(ctx)
		if r.URL.Path == "/logout" {
			next.ServeHTTP(w, authenticatedRequest)
			return
		}
		if len(memberships) != 1 {
			s.renderMembershipError(w, authenticatedRequest, session, user.User)
			return
		}
		next.ServeHTTP(w, authenticatedRequest)
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
		return
	}
	identity, ok := identityFromRequest(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)
	if err := r.ParseForm(); err != nil || !s.validCSRF(r, identity.Session) {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу.", http.StatusForbidden)
		return
	}
	s.sessions.delete(identity.Session.ID)
	s.clearSessionCookies(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) sessionFromRequest(r *http.Request) (serverSession, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return serverSession{}, false
	}
	return s.sessions.get(cookie.Value)
}

func (s *Server) ensurePreAuthSession(w http.ResponseWriter, r *http.Request) (serverSession, error) {
	if session, ok := s.sessionFromRequest(r); ok && session.UserID == "" {
		s.setSessionCookies(w, r, session)
		return session, nil
	}
	session, err := s.sessions.create("")
	if err != nil {
		return serverSession{}, err
	}
	s.setSessionCookies(w, r, session)
	return session, nil
}

func (s *Server) validCSRF(r *http.Request, session serverSession) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(session.CSRFToken)) == 1 &&
		subtle.ConstantTimeCompare([]byte(r.FormValue("csrf_token")), []byte(session.CSRFToken)) == 1
}

func (s *Server) renderMembershipError(w http.ResponseWriter, r *http.Request, session serverSession, user model.User) {
	s.render(w, r, "login.html", http.StatusForbidden, loginPage{
		pageBase: pageBase{
			Title: "Требуется назначить компанию",
			Error: "Для доступа должна быть назначена ровно одна компания. Обратитесь к администратору.",
			CSRF:  session.CSRFToken, Authenticated: true, UserName: user.Name,
		},
	})
}

func (s *Server) setSessionCookies(w http.ResponseWriter, r *http.Request, session serverSession) {
	secure := s.secureCookie(r)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: session.ID, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		Expires: session.ExpiresAt, MaxAge: int(sessionLifetime.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName, Value: session.CSRFToken, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: int(sessionLifetime.Seconds()),
	})
}

func (s *Server) clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	secure := s.secureCookie(r)
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", HttpOnly: true, Secure: secure,
			SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
}

func (s *Server) secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(os.Getenv("APP_ENV"), "production")
}

func (s *Server) applyPageSecurity(r *http.Request, data any) any {
	csrf := s.csrfToken
	if r != nil {
		if session, ok := s.sessionFromRequest(r); ok {
			csrf = session.CSRFToken
		}
	}
	var authenticated bool
	var userName string
	if r != nil {
		if identity, ok := identityFromRequest(r); ok {
			authenticated, userName = true, identity.User.Name
		}
	}
	set := func(base *pageBase) {
		base.CSRF = csrf
		base.Authenticated = authenticated
		base.UserName = userName
	}
	switch page := data.(type) {
	case indexPage:
		set(&page.pageBase)
		return page
	case resultPage:
		set(&page.pageBase)
		return page
	case lifecyclePage:
		set(&page.pageBase)
		return page
	case loginPage:
		page.Authenticated = authenticated
		page.UserName = userName
		return page
	case registerPage:
		page.Authenticated = authenticated
		page.UserName = userName
		return page
	case equipmentListPage:
		set(&page.pageBase)
		return page
	case equipmentFormPage:
		set(&page.pageBase)
		return page
	case materialListPage:
		set(&page.pageBase)
		return page
	case materialFormPage:
		set(&page.pageBase)
		return page
	case stockBlankListPage:
		set(&page.pageBase)
		return page
	case stockBlankFormPage:
		set(&page.pageBase)
		return page
	case companyPage:
		set(&page.pageBase)
		return page
	case workspacePage:
		set(&page.pageBase)
		return page
	default:
		return data
	}
}

func companyIDForRequest(s *Server, r *http.Request) string {
	if identity, ok := identityFromRequest(r); ok {
		return identity.Company.ID
	}
	return s.companyID
}

func (s *Server) rejectUnauthorizedMutation(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	identity, ok := identityFromRequest(r)
	if !ok {
		http.Error(w, "Требуется авторизация.", http.StatusUnauthorized)
		return true
	}
	if r.URL.Path == "/company/save" {
		if identity.Role == model.CompanyRoleOwner || identity.Role == model.CompanyRoleAdmin {
			return false
		}
		http.Error(w, "Недостаточно прав для изменения профиля компании.", http.StatusForbidden)
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/revisions/") && strings.HasSuffix(r.URL.Path, "/calculate") {
		switch identity.Role {
		case model.CompanyRoleOwner, model.CompanyRoleAdmin, model.CompanyRoleManager, model.CompanyRoleOperator:
			return false
		default:
			http.Error(w, "Недостаточно прав для расчёта сметы.", http.StatusForbidden)
			return true
		}
	}
	if r.URL.Path == "/chat" ||
		(strings.HasPrefix(r.URL.Path, "/revisions/") && strings.HasSuffix(r.URL.Path, "/estimates")) ||
		(strings.HasPrefix(r.URL.Path, "/estimates/") && (strings.HasSuffix(r.URL.Path, "/calculate") || strings.HasSuffix(r.URL.Path, "/assist"))) {
		switch identity.Role {
		case model.CompanyRoleOwner, model.CompanyRoleAdmin, model.CompanyRoleManager, model.CompanyRoleOperator:
			return false
		default:
			http.Error(w, "Недостаточно прав для создания или расчёта сметы.", http.StatusForbidden)
			return true
		}
	}
	if strings.HasPrefix(r.URL.Path, "/estimates/") {
		if identity.Role == model.CompanyRoleOwner ||
			identity.Role == model.CompanyRoleAdmin ||
			identity.Role == model.CompanyRoleManager {
			return false
		}
		http.Error(w, "Недостаточно прав для изменения сметы.", http.StatusForbidden)
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/projects/") ||
		strings.HasPrefix(r.URL.Path, "/revisions/") ||
		strings.HasPrefix(r.URL.Path, "/equipment/") ||
		strings.HasPrefix(r.URL.Path, "/materials/") ||
		strings.HasPrefix(r.URL.Path, "/consumables/") ||
		strings.HasPrefix(r.URL.Path, "/orders/") {
		if identity.Role == model.CompanyRoleOwner ||
			identity.Role == model.CompanyRoleAdmin ||
			identity.Role == model.CompanyRoleManager {
			return false
		}
		http.Error(w, "Недостаточно прав для изменения данных компании.", http.StatusForbidden)
		return true
	}
	return false
}

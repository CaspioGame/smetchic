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

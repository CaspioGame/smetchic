package web

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"smetchic/application"
	"smetchic/model"
)

type lifecyclePage struct {
	pageBase
	Company    model.Company
	View       string
	Projects   []application.ProjectRecord
	Project    application.ProjectRecord
	Parts      []application.PartRecord
	Part       application.PartRecord
	Revisions  []application.PartRevisionRecord
	Revision   application.PartRevisionRecord
	Estimates  []application.EstimateRecord
	Estimate   application.EstimateRecord
	Versions   []application.EstimateVersionRecord
	Version    application.EstimateVersionRecord
	Result     model.Estimate
	Equipment  []model.Equipment
	Markup     string
	FormName   string
	FormDesc   string
	FormAction string
	FormSubmit string
}

func (s *Server) serveLifecycle(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/projects") &&
		!strings.HasPrefix(r.URL.Path, "/parts/") &&
		!strings.HasPrefix(r.URL.Path, "/revisions/") &&
		!strings.HasPrefix(r.URL.Path, "/estimates/") {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Метод не поддерживается.", http.StatusMethodNotAllowed)
		return true
	}

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/projects" && r.Method == http.MethodGet:
		s.renderProjects(w, r)
	case r.URL.Path == "/projects/new" && r.Method == http.MethodGet:
		s.setCSRFCookie(w, r)
		page := s.lifecycleBase(r, "Новый проект", "projects", "project-form")
		page.FormAction, page.FormSubmit = "/projects/create", "Создать проект"
		s.render(w, r, "lifecycle.html", http.StatusOK, page)
	case r.URL.Path == "/projects/create" && r.Method == http.MethodPost:
		s.createProject(w, r)
	case len(segments) == 3 && segments[0] == "projects" && segments[2] == "edit" && r.Method == http.MethodGet:
		s.editProject(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "projects" && segments[2] == "save" && r.Method == http.MethodPost:
		s.updateProject(w, r, segments[1])
	case len(segments) == 2 && segments[0] == "projects" && r.Method == http.MethodGet:
		s.renderProject(w, r, segments[1])
	case len(segments) == 4 && segments[0] == "projects" && segments[2] == "parts" && segments[3] == "upload" && r.Method == http.MethodPost:
		s.uploadPartRevision(w, r, segments[1])
	case len(segments) == 2 && segments[0] == "parts" && r.Method == http.MethodGet:
		s.renderPart(w, r, segments[1])
	case len(segments) == 2 && segments[0] == "revisions" && r.Method == http.MethodGet:
		s.renderRevision(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "revisions" && segments[2] == "source" && r.Method == http.MethodGet:
		s.downloadRevisionSource(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "revisions" && segments[2] == "calculate" && r.Method == http.MethodPost:
		s.calculateRevision(w, r, segments[1])
	case len(segments) == 2 && segments[0] == "estimates" && r.Method == http.MethodGet:
		s.renderEstimate(w, r, segments[1])
	case len(segments) == 4 && segments[0] == "estimates" && segments[2] == "versions" && r.Method == http.MethodGet:
		s.renderEstimateVersion(w, r, segments[1], segments[3])
	default:
		http.NotFound(w, r)
	}
	return true
}

func (s *Server) renderProjects(w http.ResponseWriter, r *http.Request) {
	s.setCSRFCookie(w, r)
	projects, err := s.lifecycle.ListProjects(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.writeInternalError(w, "list PostgreSQL projects", err)
		return
	}
	page := s.lifecycleBase(r, "Проекты", "projects", "projects")
	page.Projects = projects
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	project, err := s.lifecycle.CreateProject(r.Context(), companyIDForRequest(s, r), name, description)
	if err != nil {
		s.renderLifecycleError(w, r, "Новый проект", "project-form", err, name, description)
		return
	}
	http.Redirect(w, r, "/projects/"+project.ID, http.StatusSeeOther)
}

func (s *Server) editProject(w http.ResponseWriter, r *http.Request, projectID string) {
	s.setCSRFCookie(w, r)
	project, err := s.lifecycle.GetProject(r.Context(), companyIDForRequest(s, r), projectID)
	if err != nil {
		s.lifecycleError(w, r, "load project for edit", err)
		return
	}
	page := s.lifecycleBase(r, "Редактирование проекта", "projects", "project-form")
	page.Project = project
	page.FormName, page.FormDesc = project.Name, project.Description
	page.FormAction, page.FormSubmit = "/projects/"+project.ID+"/save", "Сохранить изменения"
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request, projectID string) {
	if !s.parseMutationForm(w, r) {
		return
	}
	name, description := r.FormValue("name"), r.FormValue("description")
	project, err := s.lifecycle.UpdateProject(r.Context(), companyIDForRequest(s, r), projectID, name, description)
	if err != nil {
		page := s.lifecycleBase(r, "Редактирование проекта", "projects", "project-form")
		page.Project = application.ProjectRecord{ID: projectID}
		page.FormName, page.FormDesc = name, description
		page.FormAction, page.FormSubmit = "/projects/"+projectID+"/save", "Сохранить изменения"
		page.Error = lifecyclePublicError(err)
		if !isLifecycleValidationError(err) && !errors.Is(err, application.ErrNotFound) {
			page.Error = "Не удалось сохранить проект. Повторите попытку."
			s.render(w, r, "lifecycle.html", http.StatusInternalServerError, page)
			return
		}
		status := http.StatusBadRequest
		if errors.Is(err, application.ErrNotFound) {
			status = http.StatusNotFound
		}
		s.render(w, r, "lifecycle.html", status, page)
		return
	}
	http.Redirect(w, r, "/projects/"+project.ID, http.StatusSeeOther)
}

func (s *Server) renderProject(w http.ResponseWriter, r *http.Request, projectID string) {
	s.setCSRFCookie(w, r)
	project, err := s.lifecycle.GetProject(r.Context(), companyIDForRequest(s, r), projectID)
	if err != nil {
		s.lifecycleError(w, r, "project", err)
		return
	}
	parts, err := s.lifecycle.ListParts(r.Context(), companyIDForRequest(s, r), projectID)
	if err != nil {
		s.writeInternalError(w, "list PostgreSQL project parts", err)
		return
	}
	page := s.lifecycleBase(r, project.Name, "projects", "project")
	page.Project, page.Parts = project, parts
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) uploadPartRevision(w http.ResponseWriter, r *http.Request, projectID string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(memoryFormSize); err != nil {
		http.Error(w, "Не удалось прочитать форму или файл превышает ограничение 2 МБ.", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if !s.validCSRFFromForm(r) {
		http.Error(w, "Форма устарела или не прошла проверку безопасности. Обновите страницу.", http.StatusForbidden)
		return
	}
	file, header, err := r.FormFile("part_file")
	if err != nil {
		s.renderProjectLifecycleError(w, r, projectID, application.Invalid("Выберите файл .part.", nil))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil || len(content) > maxUploadSize {
		http.Error(w, "Не удалось прочитать файл .part или он превышает ограничение 2 МБ.", http.StatusBadRequest)
		return
	}
	_, revision, err := s.lifecycle.UploadRevision(r.Context(), companyIDForRequest(s, r), projectID, application.RevisionUpload{
		PartName: r.FormValue("part_name"), PartDescription: r.FormValue("part_description"),
		SourceFilename: header.Filename, SourceContent: content,
	})
	if err != nil {
		s.renderProjectLifecycleError(w, r, projectID, err)
		return
	}
	http.Redirect(w, r, "/revisions/"+revision.ID, http.StatusSeeOther)
}

func (s *Server) renderPart(w http.ResponseWriter, r *http.Request, partID string) {
	s.setCSRFCookie(w, r)
	part, err := s.lifecycle.GetPart(r.Context(), companyIDForRequest(s, r), partID)
	if err != nil {
		s.lifecycleError(w, r, "get part", err)
		return
	}
	revisions, err := s.lifecycle.ListPartRevisions(r.Context(), companyIDForRequest(s, r), partID)
	if err != nil {
		s.writeInternalError(w, "list part revisions", err)
		return
	}
	project, err := s.lifecycle.GetProject(r.Context(), companyIDForRequest(s, r), part.ProjectID)
	if err != nil {
		s.writeInternalError(w, "get part project", err)
		return
	}
	page := s.lifecycleBase(r, part.Name, "projects", "part")
	page.Part, page.Project, page.Revisions = part, project, revisions
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) renderRevision(w http.ResponseWriter, r *http.Request, revisionID string) {
	s.setCSRFCookie(w, r)
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), companyIDForRequest(s, r), revisionID)
	if err != nil {
		s.lifecycleError(w, r, "get part revision", err)
		return
	}
	equipment, err := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load PostgreSQL equipment for estimate", err)
		return
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load PostgreSQL company settings for estimate", err)
		return
	}
	page := s.lifecycleBase(r, detail.Part.Name+" — ревизия", "projects", "revision")
	page.Project, page.Part, page.Revision = detail.Project, detail.Part, detail.Revision
	page.Estimates, page.Equipment = detail.Estimates, activeEquipment(equipment)
	page.Markup = formatNumber(settings.DefaultMarkupPercent)
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) downloadRevisionSource(w http.ResponseWriter, r *http.Request, revisionID string) {
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), companyIDForRequest(s, r), revisionID)
	if err != nil {
		s.lifecycleError(w, r, "download part revision source", err)
		return
	}
	filename := filepath.Base(detail.Revision.SourceFilename)
	if filename == "" || filename == "." {
		filename = "part.part"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, detail.Revision.SourceContent); err != nil {
		log.Printf("write part revision source response: %v", err)
	}
}

func (s *Server) calculateRevision(w http.ResponseWriter, r *http.Request, revisionID string) {
	if !s.parseMutationForm(w, r) {
		return
	}
	markup, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("markup_percent")), 64)
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Укажите корректный процент наценки.", nil))
		return
	}
	_, version, err := s.lifecycle.Calculate(
		r.Context(), companyIDForRequest(s, r), revisionID, r.FormValue("estimate_name"),
		r.FormValue("equipment_id"), markup,
	)
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, err)
		return
	}
	estimate, err := s.lifecycle.GetEstimate(r.Context(), companyIDForRequest(s, r), version.EstimateID)
	if err != nil {
		s.writeInternalError(w, "get newly saved estimate", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/estimates/%s/versions/%s", estimate.ID, version.ID), http.StatusSeeOther)
}

func (s *Server) renderEstimate(w http.ResponseWriter, r *http.Request, estimateID string) {
	s.setCSRFCookie(w, r)
	estimate, err := s.lifecycle.GetEstimate(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.lifecycleError(w, r, "get estimate", err)
		return
	}
	versions, err := s.lifecycle.ListEstimateVersions(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.writeInternalError(w, "list estimate versions", err)
		return
	}
	page := s.lifecycleBase(r, estimate.Name, "projects", "estimate")
	page.Estimate, page.Versions = estimate, versions
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) renderEstimateVersion(w http.ResponseWriter, r *http.Request, estimateID, versionID string) {
	s.setCSRFCookie(w, r)
	detail, err := s.lifecycle.GetEstimateVersion(r.Context(), companyIDForRequest(s, r), estimateID, versionID)
	if err != nil {
		s.lifecycleError(w, r, "get estimate version", err)
		return
	}
	var result model.Estimate
	if err := json.Unmarshal(detail.Version.ResultSnapshot, &result); err != nil {
		s.writeInternalError(w, "decode saved estimate result", err)
		return
	}
	page := s.lifecycleBase(r, detail.Estimate.Name, "projects", "version")
	page.Estimate, page.Version, page.Result = detail.Estimate, detail.Version, result
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) lifecycleBase(r *http.Request, title, active, view string) lifecyclePage {
	company := s.company
	if identity, ok := identityFromRequest(r); ok {
		company = identity.Company
	}
	return lifecyclePage{
		pageBase: pageBase{
			Title: title, Active: active, CSRF: s.csrfToken, LifecycleEnabled: true,
		},
		Company: company, View: view,
	}
}

func (s *Server) renderLifecycleError(w http.ResponseWriter, r *http.Request, title, view string, err error, name, description string) {
	status := http.StatusBadRequest
	if errors.Is(err, application.ErrNotFound) {
		status = http.StatusNotFound
	} else if !isLifecycleValidationError(err) {
		status = http.StatusInternalServerError
	}
	page := s.lifecycleBase(r, title, "projects", view)
	page.Error, page.FormName, page.FormDesc = lifecyclePublicError(err), name, description
	if view == "project-form" {
		page.FormAction, page.FormSubmit = "/projects/create", "Создать проект"
	}
	s.render(w, r, "lifecycle.html", status, page)
}

func (s *Server) renderProjectLifecycleError(w http.ResponseWriter, r *http.Request, projectID string, err error) {
	project, getErr := s.lifecycle.GetProject(r.Context(), companyIDForRequest(s, r), projectID)
	if getErr != nil {
		s.lifecycleError(w, r, "load project after validation error", getErr)
		return
	}
	parts, getErr := s.lifecycle.ListParts(r.Context(), companyIDForRequest(s, r), projectID)
	if getErr != nil {
		s.writeInternalError(w, "load project parts after validation error", getErr)
		return
	}
	page := s.lifecycleBase(r, project.Name, "projects", "project")
	page.Project, page.Parts = project, parts
	page.Error = lifecyclePublicError(err)
	status := http.StatusBadRequest
	if errors.Is(err, application.ErrNotFound) {
		status = http.StatusNotFound
	} else if !isLifecycleValidationError(err) {
		status = http.StatusInternalServerError
	}
	s.render(w, r, "lifecycle.html", status, page)
}

func (s *Server) renderRevisionLifecycleError(w http.ResponseWriter, r *http.Request, revisionID string, err error) {
	detail, getErr := s.lifecycle.GetRevisionEstimate(r.Context(), companyIDForRequest(s, r), revisionID)
	if getErr != nil {
		s.lifecycleError(w, r, "load revision after validation error", getErr)
		return
	}
	equipment, getErr := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load PostgreSQL equipment after validation error", getErr)
		return
	}
	settings, getErr := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load PostgreSQL settings after validation error", getErr)
		return
	}
	page := s.lifecycleBase(r, detail.Part.Name+" — ревизия", "projects", "revision")
	page.Project, page.Part, page.Revision = detail.Project, detail.Part, detail.Revision
	page.Estimates, page.Equipment = detail.Estimates, activeEquipment(equipment)
	page.Markup = formatNumber(settings.DefaultMarkupPercent)
	page.Error = lifecyclePublicError(err)
	status := http.StatusBadRequest
	if errors.Is(err, application.ErrNotFound) {
		status = http.StatusNotFound
	} else if !isLifecycleValidationError(err) {
		status = http.StatusInternalServerError
	}
	s.render(w, r, "lifecycle.html", status, page)
}

func lifecyclePublicError(err error) string {
	var validationError *application.ValidationError
	if errors.As(err, &validationError) {
		return validationError.Error()
	}
	if errors.Is(err, application.ErrNotFound) {
		return "Запись не найдена или недоступна в текущей компании."
	}
	log.Printf("PostgreSQL lifecycle operation failed: %v", err)
	return "Не удалось выполнить операцию. Проверьте данные и повторите попытку."
}

func isLifecycleValidationError(err error) bool {
	var validationError *application.ValidationError
	return errors.As(err, &validationError)
}

func (s *Server) lifecycleError(w http.ResponseWriter, r *http.Request, action string, err error) {
	if errors.Is(err, application.ErrNotFound) {
		http.Error(w, "Запись не найдена или недоступна в текущей компании.", http.StatusNotFound)
		return
	}
	log.Printf("%s: %v", action, err)
	http.Error(w, "Не удалось загрузить данные. Попробуйте обновить страницу.", http.StatusInternalServerError)
}

func (s *Server) validCSRFFromForm(r *http.Request) bool {
	if s.authentication != nil {
		session, ok := s.sessionFromRequest(r)
		return ok && s.validCSRF(r, session)
	}
	cookie, err := r.Cookie("cnc_csrf")
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.csrfToken)) == 1 &&
		subtle.ConstantTimeCompare([]byte(r.FormValue("csrf_token")), []byte(s.csrfToken)) == 1
}

func activeEquipment(items []model.Equipment) []model.Equipment {
	active := make([]model.Equipment, 0, len(items))
	for _, item := range items {
		if item.Status == model.EquipmentStatusActive {
			active = append(active, item)
		}
	}
	return active
}

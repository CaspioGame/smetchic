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
	View      string
	Projects  []application.ProjectRecord
	Project   application.ProjectRecord
	Parts     []application.PartRecord
	Part      application.PartRecord
	Revisions []application.PartRevisionRecord
	Revision  application.PartRevisionRecord
	Estimates []application.EstimateRecord
	Estimate  application.EstimateRecord
	Versions  []application.EstimateVersionRecord
	Version   application.EstimateVersionRecord
	Result    model.Estimate
	Equipment []model.Equipment
	Markup    string
	FormName  string
	FormDesc  string
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
		s.setCSRFCookie(w)
		page := s.lifecycleBase("Новый проект", "projects", "project-form")
		s.render(w, "lifecycle.html", http.StatusOK, page)
	case r.URL.Path == "/projects/create" && r.Method == http.MethodPost:
		s.createProject(w, r)
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
	s.setCSRFCookie(w)
	projects, err := s.lifecycle.ListProjects(r.Context(), s.companyID)
	if err != nil {
		s.writeInternalError(w, "list PostgreSQL projects", err)
		return
	}
	page := s.lifecycleBase("Проекты", "projects", "projects")
	page.Projects = projects
	s.render(w, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	if !s.parseMutationForm(w, r) {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	project, err := s.lifecycle.CreateProject(r.Context(), s.companyID, name, description)
	if err != nil {
		s.renderLifecycleError(w, "Новый проект", "project-form", err, name, description)
		return
	}
	http.Redirect(w, r, "/projects/"+project.ID, http.StatusSeeOther)
}

func (s *Server) renderProject(w http.ResponseWriter, r *http.Request, projectID string) {
	s.setCSRFCookie(w)
	project, err := s.lifecycle.GetProject(r.Context(), s.companyID, projectID)
	if err != nil {
		s.lifecycleError(w, "project", err)
		return
	}
	parts, err := s.lifecycle.ListParts(r.Context(), s.companyID, projectID)
	if err != nil {
		s.writeInternalError(w, "list PostgreSQL project parts", err)
		return
	}
	page := s.lifecycleBase(project.Name, "projects", "project")
	page.Project, page.Parts = project, parts
	s.render(w, "lifecycle.html", http.StatusOK, page)
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
	_, revision, err := s.lifecycle.UploadRevision(r.Context(), s.companyID, projectID, application.RevisionUpload{
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
	s.setCSRFCookie(w)
	part, err := s.lifecycle.GetPart(r.Context(), s.companyID, partID)
	if err != nil {
		s.lifecycleError(w, "get part", err)
		return
	}
	revisions, err := s.lifecycle.ListPartRevisions(r.Context(), s.companyID, partID)
	if err != nil {
		s.writeInternalError(w, "list part revisions", err)
		return
	}
	project, err := s.lifecycle.GetProject(r.Context(), s.companyID, part.ProjectID)
	if err != nil {
		s.writeInternalError(w, "get part project", err)
		return
	}
	page := s.lifecycleBase(part.Name, "projects", "part")
	page.Part, page.Project, page.Revisions = part, project, revisions
	s.render(w, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) renderRevision(w http.ResponseWriter, r *http.Request, revisionID string) {
	s.setCSRFCookie(w)
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), s.companyID, revisionID)
	if err != nil {
		s.lifecycleError(w, "get part revision", err)
		return
	}
	snapshot, err := s.store.Snapshot(s.companyID)
	if err != nil {
		s.writeInternalError(w, "load company settings for estimate", err)
		return
	}
	page := s.lifecycleBase(detail.Part.Name+" — ревизия", "projects", "revision")
	page.Project, page.Part, page.Revision = detail.Project, detail.Part, detail.Revision
	page.Estimates, page.Equipment = detail.Estimates, activeEquipment(snapshot.Equipment)
	page.Markup = formatNumber(snapshot.Settings.DefaultMarkupPercent)
	s.render(w, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) downloadRevisionSource(w http.ResponseWriter, r *http.Request, revisionID string) {
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), s.companyID, revisionID)
	if err != nil {
		s.lifecycleError(w, "download part revision source", err)
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
		r.Context(), s.companyID, revisionID, r.FormValue("estimate_name"),
		r.FormValue("equipment_id"), markup,
	)
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, err)
		return
	}
	estimate, err := s.lifecycle.GetEstimate(r.Context(), s.companyID, version.EstimateID)
	if err != nil {
		s.writeInternalError(w, "get newly saved estimate", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/estimates/%s/versions/%s", estimate.ID, version.ID), http.StatusSeeOther)
}

func (s *Server) renderEstimate(w http.ResponseWriter, r *http.Request, estimateID string) {
	s.setCSRFCookie(w)
	estimate, err := s.lifecycle.GetEstimate(r.Context(), s.companyID, estimateID)
	if err != nil {
		s.lifecycleError(w, "get estimate", err)
		return
	}
	versions, err := s.lifecycle.ListEstimateVersions(r.Context(), s.companyID, estimateID)
	if err != nil {
		s.writeInternalError(w, "list estimate versions", err)
		return
	}
	page := s.lifecycleBase(estimate.Name, "projects", "estimate")
	page.Estimate, page.Versions = estimate, versions
	s.render(w, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) renderEstimateVersion(w http.ResponseWriter, r *http.Request, estimateID, versionID string) {
	s.setCSRFCookie(w)
	detail, err := s.lifecycle.GetEstimateVersion(r.Context(), s.companyID, estimateID, versionID)
	if err != nil {
		s.lifecycleError(w, "get estimate version", err)
		return
	}
	var result model.Estimate
	if err := json.Unmarshal(detail.Version.ResultSnapshot, &result); err != nil {
		s.writeInternalError(w, "decode saved estimate result", err)
		return
	}
	page := s.lifecycleBase(detail.Estimate.Name, "projects", "version")
	page.Estimate, page.Version, page.Result = detail.Estimate, detail.Version, result
	s.render(w, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) lifecycleBase(title, active, view string) lifecyclePage {
	return lifecyclePage{
		pageBase: pageBase{
			Title: title, Active: active, CSRF: s.csrfToken, LifecycleEnabled: true,
		},
		View: view,
	}
}

func (s *Server) renderLifecycleError(w http.ResponseWriter, title, view string, err error, name, description string) {
	status := http.StatusBadRequest
	message := err.Error()
	var validationError *application.ValidationError
	if errors.Is(err, application.ErrNotFound) {
		status, message = http.StatusNotFound, "Запись не найдена или недоступна в текущей компании."
	} else if !errors.As(err, &validationError) {
		log.Printf("lifecycle request failed: %v", err)
		status, message = http.StatusInternalServerError, "Не удалось выполнить операцию. Проверьте данные и повторите попытку."
	}
	page := s.lifecycleBase(title, "projects", view)
	page.Error, page.FormName, page.FormDesc = message, name, description
	s.render(w, "lifecycle.html", status, page)
}

func (s *Server) renderProjectLifecycleError(w http.ResponseWriter, r *http.Request, projectID string, err error) {
	project, getErr := s.lifecycle.GetProject(r.Context(), s.companyID, projectID)
	if getErr != nil {
		s.lifecycleError(w, "load project after validation error", getErr)
		return
	}
	parts, getErr := s.lifecycle.ListParts(r.Context(), s.companyID, projectID)
	if getErr != nil {
		s.writeInternalError(w, "load project parts after validation error", getErr)
		return
	}
	page := s.lifecycleBase(project.Name, "projects", "project")
	page.Project, page.Parts = project, parts
	page.Error = lifecyclePublicError(err)
	s.render(w, "lifecycle.html", http.StatusBadRequest, page)
}

func (s *Server) renderRevisionLifecycleError(w http.ResponseWriter, r *http.Request, revisionID string, err error) {
	detail, getErr := s.lifecycle.GetRevisionEstimate(r.Context(), s.companyID, revisionID)
	if getErr != nil {
		s.lifecycleError(w, "load revision after validation error", getErr)
		return
	}
	snapshot, getErr := s.store.Snapshot(s.companyID)
	if getErr != nil {
		s.writeInternalError(w, "load company settings after validation error", getErr)
		return
	}
	page := s.lifecycleBase(detail.Part.Name+" — ревизия", "projects", "revision")
	page.Project, page.Part, page.Revision = detail.Project, detail.Part, detail.Revision
	page.Estimates, page.Equipment = detail.Estimates, activeEquipment(snapshot.Equipment)
	page.Markup = formatNumber(snapshot.Settings.DefaultMarkupPercent)
	page.Error = lifecyclePublicError(err)
	s.render(w, "lifecycle.html", http.StatusBadRequest, page)
}

func lifecyclePublicError(err error) string {
	var validationError *application.ValidationError
	if errors.As(err, &validationError) {
		return validationError.Error()
	}
	return "Не удалось выполнить операцию. Проверьте данные и повторите попытку."
}

func (s *Server) lifecycleError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, application.ErrNotFound) {
		http.Error(w, "Запись не найдена или недоступна в текущей компании.", http.StatusNotFound)
		return
	}
	log.Printf("%s: %v", action, err)
	http.Error(w, "Не удалось загрузить данные. Попробуйте обновить страницу.", http.StatusInternalServerError)
}

func (s *Server) validCSRFFromForm(r *http.Request) bool {
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

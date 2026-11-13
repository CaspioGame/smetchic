package web

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
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
	Company                     model.Company
	Currency                    string
	View                        string
	Projects                    []application.ProjectRecord
	Project                     application.ProjectRecord
	Order                       application.OrderRecord
	AutomaticMachineTimeMinutes float64
	UsedMachineTimeMinutes      float64
	MachineTimeSource           application.EstimateValueSource
	MachineTimeReason           string
	HasAutomaticMachineTime     bool
	PreviousVersionNumber       int
	HasPreviousVersion          bool
	IsCurrentVersion            bool
	VersionChanges              []estimateVersionChange
	CostItems                   []model.ProductionCostItem
	CostCalculations            map[string]costExplanation
	CostTotals                  []costTotalDisplay
	CostWarnings                []string
	MaterialName                string
	MaterialPriceLabel          string
	MaterialPriceNote           string
	Parts                       []application.PartRecord
	Part                        application.PartRecord
	Revisions                   []application.PartRevisionRecord
	Revision                    application.PartRevisionRecord
	Estimates                   []application.EstimateRecord
	Estimate                    application.EstimateRecord
	EstimateInput               application.EstimateInput
	Versions                    []application.EstimateVersionRecord
	Version                     application.EstimateVersionRecord
	Result                      model.Estimate
	Equipment                   []model.Equipment
	Materials                   []model.Material
	GlobalMaterials             []model.GlobalMaterial
	StockBlanks                 []model.StockBlank
	Consumables                 []model.Consumable
	GlobalTools                 []model.Consumable
	Orders                      []application.OrderRecord
	Markup                      string
	FormName                    string
	FormDesc                    string
	FormAction                  string
	FormSubmit                  string
	EstimateFormValues          map[string]string
	AssistantChanges            []string
	ChatEstimateID              string
	ChatTurns                   []estimateChatTurn
	Dialogs                     []application.ChatDialog
	DialogID                    string
	DialogRevision              int
}

type estimateVersionChange struct {
	Name   string
	Before string
	After  string
	Reason string
}

type estimateVersionSnapshot struct {
	WorkingInput application.EstimateInput `json:"working_input"`
	Options      struct {
		DocumentInput application.EstimateInput `json:"document_input"`
	} `json:"options"`
	AutomaticMachineTimeHours float64                         `json:"automatic_machine_time_hours"`
	UsedMachineTimeHours      float64                         `json:"used_machine_time_hours"`
	MachineTimeSource         application.EstimateValueSource `json:"machine_time_source"`
	MachineTimeReason         string                          `json:"machine_time_reason"`
}

func (s *Server) serveLifecycle(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/estimates" && r.URL.Path != "/chat" &&
		!strings.HasPrefix(r.URL.Path, "/projects") &&
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
	case r.URL.Path == "/chat":
		s.renderEstimateChat(w, r)
	case r.URL.Path == "/projects" && r.Method == http.MethodGet:
		s.renderProjects(w, r)
	case r.URL.Path == "/estimates" && r.Method == http.MethodGet:
		s.renderEstimates(w, r)
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
	case len(segments) == 3 && segments[0] == "revisions" && segments[2] == "estimates" && r.Method == http.MethodPost:
		s.createEstimateFromRevision(w, r, segments[1])
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
	case len(segments) == 3 && segments[0] == "estimates" && segments[2] == "edit" && r.Method == http.MethodGet:
		s.renderEstimateEdit(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "estimates" && segments[2] == "calculate" && r.Method == http.MethodPost:
		s.calculateEstimateDocument(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "estimates" && segments[2] == "assist" && r.Method == http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if s.parseMutationForm(w, r) {
			s.renderEstimateEdit(w, r, segments[1])
		}
	case len(segments) == 3 && segments[0] == "estimates" && segments[2] == "versions" && r.Method == http.MethodGet:
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

func (s *Server) renderEstimates(w http.ResponseWriter, r *http.Request) {
	estimates, err := s.lifecycle.ListCompanyEstimates(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "list company estimates", err)
		return
	}
	page := s.lifecycleBase(r, "Сметы", "estimates", "estimates")
	page.Estimates = estimates
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
		s.renderProjectLifecycleError(w, r, projectID, application.Invalid("Выберите файл детали.", nil))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil || len(content) > maxUploadSize {
		http.Error(w, "Не удалось прочитать файл или он превышает ограничение 2 МБ.", http.StatusBadRequest)
		return
	}
	_, revision, err := s.lifecycle.AttachRevision(r.Context(), companyIDForRequest(s, r), projectID, application.RevisionUpload{
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
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load company materials for estimate", err)
		return
	}
	globalMaterials, err := s.lifecycle.ListGlobalMaterials(r.Context())
	if err != nil {
		s.lifecycleError(w, r, "load global reference materials for estimate", err)
		return
	}
	stockBlanks, err := s.lifecycle.ListStockBlanks(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load company stock blanks for estimate", err)
		return
	}
	consumables, err := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load company tooling for estimate", err)
		return
	}
	orders, err := s.lifecycle.ListOrders(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load orders for estimate creation", err)
		return
	}
	page := s.lifecycleBase(r, detail.Part.Name+" — ревизия", "projects", "revision")
	page.Project, page.Part, page.Revision = detail.Project, detail.Part, detail.Revision
	page.Estimates, page.Equipment = detail.Estimates, activeEquipment(equipment)
	page.Materials, page.GlobalMaterials = materials, globalMaterials
	page.StockBlanks, page.Consumables, page.Orders = stockBlanks, consumables, orders
	if !s.loadEstimateGlobalTools(w, r, &page) {
		return
	}
	page.Markup = formatNumber(settings.DefaultMarkupPercent)
	page.Currency = settings.Currency
	page.EstimateFormValues = createEstimateFormValues(page.Markup)
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) createEstimateFromRevision(w http.ResponseWriter, r *http.Request, revisionID string) {
	if !s.parseMutationForm(w, r) {
		return
	}
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), companyIDForRequest(s, r), revisionID)
	if err != nil {
		s.lifecycleError(w, r, "load revision for estimate creation", err)
		return
	}
	input, err := parseEstimateInput(r, application.EstimateInput{PartRevisionID: revisionID, ProjectID: detail.Project.ID})
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Проверьте параметры новой сметы.", err))
		return
	}
	estimate, err := s.lifecycle.CreateEstimate(r.Context(), companyIDForRequest(s, r), input)
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, err)
		return
	}
	http.Redirect(w, r, "/estimates/"+estimate.ID+"/edit", http.StatusSeeOther)
}

func (s *Server) downloadRevisionSource(w http.ResponseWriter, r *http.Request, revisionID string) {
	detail, err := s.lifecycle.GetRevisionEstimate(r.Context(), companyIDForRequest(s, r), revisionID)
	if err != nil {
		s.lifecycleError(w, r, "download part revision source", err)
		return
	}
	filename := filepath.Base(detail.Revision.SourceFilename)
	if filename == "" || filename == "." {
		filename = "part"
	}
	source, err := detail.Revision.SourceBytes()
	if err != nil {
		s.writeInternalError(w, "decode part revision source", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(source); err != nil {
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
	quantity := 1
	if strings.TrimSpace(r.FormValue("quantity")) != "" {
		quantity, err = strconv.Atoi(strings.TrimSpace(r.FormValue("quantity")))
		if err != nil {
			s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Укажите целое количество деталей.", err))
			return
		}
	}
	setupMinutes, err := parseOptionalFloat(r.FormValue("setup_time_minutes"))
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Укажите корректное время наладки.", err))
		return
	}
	programmingHours, err := parseOptionalFloat(r.FormValue("programming_hours"))
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Укажите корректное время программирования.", err))
		return
	}
	toolUsageMinutes, err := parseOptionalFloat(r.FormValue("tool_usage_minutes"))
	if err != nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Укажите корректный ресурс инструмента.", err))
		return
	}
	machineTime, err := parseMachineTime(r)
	if err != nil || machineTime == nil {
		s.renderRevisionLifecycleError(w, r, revisionID, application.Invalid("Введите время обработки одной детали в минутах.", err))
		return
	}
	_, version, err := s.lifecycle.CalculateWithProductionEconomics(
		r.Context(), companyIDForRequest(s, r), revisionID, r.FormValue("estimate_name"),
		r.FormValue("equipment_id"), markup, application.ProductionEstimateOptions{
			Quantity: quantity, SetupTimeMinutes: setupMinutes, ProgrammingHours: programmingHours,
			MaterialID: r.FormValue("material_id"), StockBlankID: r.FormValue("stock_blank_id"),
			GlobalMaterialID: r.FormValue("global_material_id"),
			ConsumableID:     r.FormValue("consumable_id"), ToolUsageMinutes: toolUsageMinutes,
			MachineTimeOverride: machineTime,
		})
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

func parseOptionalFloat(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func (s *Server) renderEstimate(w http.ResponseWriter, r *http.Request, estimateID string) {
	s.setCSRFCookie(w, r)
	document, err := s.lifecycle.GetEstimateDocument(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.lifecycleError(w, r, "get estimate", err)
		return
	}
	page := s.lifecycleBase(r, document.Estimate.Number+" — "+document.Estimate.Name, "estimates", "estimate")
	page.Estimate, page.Versions = document.Estimate, document.Versions
	page.Project, page.Part, page.Revision, page.Order = document.Project, document.Part, document.Revision, document.Order
	if document.Estimate.CurrentVersionID != "" {
		detail, err := s.lifecycle.GetEstimateVersion(
			r.Context(), companyIDForRequest(s, r), estimateID, document.Estimate.CurrentVersionID,
		)
		if err != nil {
			s.lifecycleError(w, r, "load current estimate version", err)
			return
		}
		page.Result, err = decodeEstimateResult(detail.Version.ResultSnapshot)
		if err != nil {
			s.writeInternalError(w, "decode current estimate result", err)
			return
		}
		page.Version = detail.Version
		if err := prepareEstimateCostDisplay(&page, detail.Version.SettingsSnapshot, detail.Version.InputSnapshot); err != nil {
			s.writeInternalError(w, "decode current estimate material snapshot", err)
			return
		}
		page.Currency = detail.Version.Currency
		if err := setEstimateTimeDisplay(&page, detail.Version.InputSnapshot); err != nil {
			s.writeInternalError(w, "decode current estimate time snapshot", err)
			return
		}
	}
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func (s *Server) renderEstimateEdit(w http.ResponseWriter, r *http.Request, estimateID string) {
	s.setCSRFCookie(w, r)
	document, err := s.lifecycle.GetEstimateDocument(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.lifecycleError(w, r, "load estimate for editing", err)
		return
	}
	input, err := decodeWorkingEstimateInput(document.Estimate.WorkingInput)
	if err != nil {
		s.writeInternalError(w, "decode working estimate input", err)
		return
	}
	var currentVersion *application.EstimateVersionRecord
	for index := range document.Versions {
		if document.Versions[index].ID == document.Estimate.CurrentVersionID {
			currentVersion = &document.Versions[index]
			break
		}
	}
	if currentVersion == nil && len(document.Versions) > 0 {
		currentVersion = &document.Versions[0]
	}
	var snapshot estimateVersionSnapshot
	if currentVersion != nil {
		snapshot, err = decodeEstimateVersionSnapshot(currentVersion.InputSnapshot)
		if err != nil {
			s.writeInternalError(w, "decode estimate snapshot for editing", err)
			return
		}
		if input.Name == "" {
			input = snapshot.WorkingInput
		}
	}
	settings, err := s.lifecycle.GetCompanySettings(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load settings for estimate form", err)
		return
	}
	if input.Name == "" {
		input = application.EstimateInput{
			Name: document.Estimate.Name, ProjectID: document.Estimate.ProjectID,
			OrderID: document.Estimate.OrderID, PartRevisionID: document.Estimate.PartRevisionID,
			Quantity: document.Estimate.Quantity, MarkupSource: application.EstimateSourceCompanyData,
		}
		input.MarkupPercent = settings.DefaultMarkupPercent
	}
	equipment, err := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load equipment for estimate form", err)
		return
	}
	materials, err := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load materials for estimate form", err)
		return
	}
	globalMaterials, err := s.lifecycle.ListGlobalMaterials(r.Context())
	if err != nil {
		s.lifecycleError(w, r, "load reference materials for estimate form", err)
		return
	}
	stockBlanks, err := s.lifecycle.ListStockBlanks(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load stock blanks for estimate form", err)
		return
	}
	consumables, err := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load tooling for estimate form", err)
		return
	}
	orders, err := s.lifecycle.ListOrders(r.Context(), companyIDForRequest(s, r))
	if err != nil {
		s.lifecycleError(w, r, "load orders for estimate form", err)
		return
	}
	page := s.lifecycleBase(r, document.Estimate.Number+" — редактирование", "estimates", "estimate-edit")
	page.Estimate, page.EstimateInput = document.Estimate, input
	page.Currency = settings.Currency
	page.Project, page.Part, page.Revision = document.Project, document.Part, document.Revision
	page.Equipment, page.Materials, page.GlobalMaterials = activeEquipment(equipment), materials, globalMaterials
	page.StockBlanks, page.Consumables, page.Orders = stockBlanks, consumables, orders
	if !s.loadEstimateGlobalTools(w, r, &page) {
		return
	}
	if currentVersion != nil {
		page.AutomaticMachineTimeMinutes = snapshot.AutomaticMachineTimeHours * 60
		page.UsedMachineTimeMinutes = snapshot.UsedMachineTimeHours * 60
		page.MachineTimeSource, page.MachineTimeReason = snapshot.MachineTimeSource, snapshot.MachineTimeReason
		page.HasAutomaticMachineTime = true
		page.Currency = currentVersion.Currency
	}
	page.EstimateFormValues = estimateInputFormValues(
		input, page.AutomaticMachineTimeMinutes, page.UsedMachineTimeMinutes, page.HasAutomaticMachineTime,
	)
	if r.Method == http.MethodPost {
		identity, ok := identityFromRequest(r)
		if ok {
			turns := s.sessions.estimateChatTurns(identity.Session.ID, identity.Company.ID, estimateID)
			proposalID := r.PostFormValue("proposal_id")
			if dialogID := r.PostFormValue("dialog_id"); dialogID != "" {
				if repo, repoErr := s.lifecycle.ChatRepository(); repoErr == nil {
					if dialog, getErr := repo.GetChat(r.Context(), identity.Company.ID, identity.User.ID, dialogID); getErr == nil {
						var state dialogState
						if json.Unmarshal(dialog.State, &state) == nil {
							turns = state.Turns
						}
					}
				}
			}
			for _, turn := range turns {
				if proposalID != "" && turn.ID == proposalID && turn.EstimateID == estimateID && turn.VersionID == document.Estimate.CurrentVersionID {
					page.AssistantChanges, err = assistantFormChanges(page.EstimateFormValues, turn.Proposal)
					break
				}
			}
		}
		if err != nil || len(page.AssistantChanges) == 0 {
			page.Error = "Предложение чата недоступно или смета уже изменилась. Запросите новое предложение."
		}
	}
	s.render(w, r, "lifecycle.html", http.StatusOK, page)
}

func estimateRequestFormValues(r *http.Request) map[string]string {
	values := make(map[string]string)
	for _, field := range []string{
		"name", "order_id", "quantity", "equipment_id", "machine_time_hours", "machine_time_source",
		"machine_time_reason", "markup_percent", "markup_source", "markup_reason", "stock_blank_id",
		"material_id", "global_material_id", "material_weight_kg", "consumable_id", "global_tool_id", "tool_usage_minutes", "tool_usage_reason",
		"setup_time_minutes", "setup_reason", "programming_hours", "programming_reason",
		"fixed_operator_cost", "fixed_setup_cost", "fixed_programming_cost",
	} {
		values[field] = r.FormValue(field)
	}
	for _, id := range r.PostForm["estimate_module"] {
		values["module_"+id] = "1"
	}
	return values
}

func estimateInputFormValues(input application.EstimateInput, automaticMinutes, usedMinutes float64, hasAutomatic bool) map[string]string {
	values := map[string]string{
		"name": input.Name, "order_id": input.OrderID, "quantity": strconv.Itoa(input.Quantity),
		"equipment_id": input.EquipmentID, "machine_time_source": string(application.EstimateSourceUserOverride),
		"machine_time_reason": "", "markup_percent": strconv.FormatFloat(input.MarkupPercent, 'f', -1, 64),
		"markup_source": string(input.MarkupSource), "markup_reason": input.MarkupReason,
		"stock_blank_id": input.StockBlankID, "material_id": input.MaterialID,
		"global_material_id": input.GlobalMaterialID, "consumable_id": input.ConsumableID,
		"global_tool_id":     input.GlobalToolID,
		"tool_usage_minutes": strconv.FormatFloat(input.ToolUsageMinutes, 'f', -1, 64),
		"tool_usage_reason":  input.ToolUsageReason,
		"setup_time_minutes": strconv.FormatFloat(input.SetupTimeMinutes, 'f', -1, 64),
		"setup_reason":       input.SetupReason, "programming_hours": strconv.FormatFloat(input.ProgrammingHours, 'f', -1, 64),
		"programming_reason": input.ProgrammingReason,
	}
	if input.MaterialWeightKg > 0 {
		values["material_weight_kg"] = formatNumber(input.MaterialWeightKg)
	}
	for _, id := range input.ActiveModules() {
		values["module_"+id] = "1"
	}
	if labor := input.ProjectLabor; labor != nil {
		for key, fee := range map[string]*float64{
			"fixed_operator_cost": labor.Operator, "fixed_setup_cost": labor.Setup, "fixed_programming_cost": labor.Programming,
		} {
			if fee != nil {
				values[key] = formatNumber(*fee)
			}
		}
	}
	if hasAutomatic {
		values["machine_time_hours"] = strconv.FormatFloat(usedMinutes, 'f', 2, 64)
	}
	if input.MachineTime != nil {
		values["machine_time_hours"] = strconv.FormatFloat(input.MachineTime.Value*60, 'f', 2, 64)
		values["machine_time_source"] = string(input.MachineTime.Source)
		values["machine_time_reason"] = input.MachineTime.Reason
	} else if hasAutomatic {
		values["machine_time_hours"] = strconv.FormatFloat(automaticMinutes, 'f', 2, 64)
	}
	return values
}

func (s *Server) calculateEstimateDocument(w http.ResponseWriter, r *http.Request, estimateID string) {
	if !s.parseMutationForm(w, r) {
		return
	}
	current, err := s.lifecycle.GetEstimate(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.lifecycleError(w, r, "load estimate before calculation", err)
		return
	}
	var existing application.EstimateInput
	if err := json.Unmarshal(current.WorkingInput, &existing); err != nil {
		s.writeInternalError(w, "decode estimate input before calculation", err)
		return
	}
	input, err := parseEstimateInput(r, existing)
	if err != nil {
		s.renderEstimateEditError(w, r, estimateID, application.Invalid("Проверьте параметры расчёта.", err))
		return
	}
	if input.MachineTime == nil {
		s.renderEstimateEditError(w, r, estimateID, application.Invalid("Введите время обработки одной детали в минутах.", nil))
		return
	}
	_, version, err := s.lifecycle.CalculateEstimate(r.Context(), companyIDForRequest(s, r), estimateID, input)
	if err != nil {
		s.renderEstimateEditError(w, r, estimateID, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/estimates/%s/versions/%s", estimateID, version.ID), http.StatusSeeOther)
}

func parseEstimateInput(r *http.Request, existing application.EstimateInput) (application.EstimateInput, error) {
	input := existing
	values := estimateRequestFormValues(r)
	if r.PostFormValue("estimate_modules_present") == "1" {
		input.Modules = append(application.EstimateModules{}, r.PostForm["estimate_module"]...)
		if err := input.Modules.Validate(); err != nil {
			return input, err
		}
	}
	if input.Modules != nil {
		filterEstimateModuleValues(values, input.Modules)
	}
	input.Name = strings.TrimSpace(values["name"])
	input.OrderID = strings.TrimSpace(values["order_id"])
	input.EquipmentID = strings.TrimSpace(values["equipment_id"])
	input.MaterialID = strings.TrimSpace(values["material_id"])
	input.GlobalMaterialID = strings.TrimSpace(values["global_material_id"])
	input.StockBlankID = strings.TrimSpace(values["stock_blank_id"])
	input.ConsumableID = strings.TrimSpace(values["consumable_id"])
	input.GlobalToolID = strings.TrimSpace(values["global_tool_id"])
	if input.MaterialID != existing.MaterialID || input.GlobalMaterialID != existing.GlobalMaterialID {
		input.MaterialSource = ""
	}
	if input.StockBlankID != existing.StockBlankID {
		input.StockBlankSource = ""
	}
	if input.ConsumableID != existing.ConsumableID || input.GlobalToolID != existing.GlobalToolID {
		input.ToolSource = ""
	}
	input.MarkupSource = application.EstimateValueSource(strings.TrimSpace(values["markup_source"]))
	input.MarkupReason = strings.TrimSpace(values["markup_reason"])
	input.SetupReason = strings.TrimSpace(values["setup_reason"])
	input.ProgrammingReason = strings.TrimSpace(values["programming_reason"])
	input.ToolUsageReason = strings.TrimSpace(values["tool_usage_reason"])
	quantity, err := strconv.Atoi(strings.TrimSpace(values["quantity"]))
	if err != nil {
		return input, fmt.Errorf("quantity must be an integer")
	}
	input.Quantity = quantity
	for _, field := range []struct {
		name string
		dst  *float64
	}{
		{"markup_percent", &input.MarkupPercent},
		{"material_weight_kg", &input.MaterialWeightKg},
		{"setup_time_minutes", &input.SetupTimeMinutes},
		{"programming_hours", &input.ProgrammingHours},
		{"tool_usage_minutes", &input.ToolUsageMinutes},
	} {
		if strings.TrimSpace(values[field.name]) == "" {
			*field.dst = 0
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(values[field.name]), 64)
		if err != nil {
			return input, fmt.Errorf("%s must be numeric", field.name)
		}
		*field.dst = value
	}
	var labor model.ProjectLabor
	input.ProjectLabor = nil
	for _, field := range []struct {
		name  string
		label string
		dst   **float64
	}{
		{"fixed_operator_cost", "Фиксированная оплата оператора", &labor.Operator},
		{"fixed_setup_cost", "Фиксированная оплата наладки", &labor.Setup},
		{"fixed_programming_cost", "Фиксированная оплата программирования", &labor.Programming},
	} {
		raw := strings.TrimSpace(values[field.name])
		if raw == "" {
			continue
		}
		fee, err := parseRequiredNumber(field.label, raw)
		if err != nil {
			return input, err
		}
		if fee < 0 {
			return input, fmt.Errorf("%s должна быть неотрицательной", field.label)
		}
		*field.dst = &fee
		input.ProjectLabor = &labor
	}
	input.MachineTime, err = parseMachineTime(r)
	return input, err
}

func parseMachineTime(r *http.Request) (*application.EstimateOverride, error) {
	raw := strings.TrimSpace(r.FormValue("machine_time_hours"))
	if raw == "" {
		return nil, nil
	}
	minutes, err := strconv.ParseFloat(raw, 64)
	if err != nil || minutes < 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return nil, fmt.Errorf("время обработки должно быть конечным неотрицательным числом")
	}
	return &application.EstimateOverride{
		Value: minutes / 60, Source: application.EstimateSourceUserOverride,
		Reason: strings.TrimSpace(r.FormValue("machine_time_reason")),
	}, nil
}

func decodeWorkingEstimateInput(data []byte) (application.EstimateInput, error) {
	var input application.EstimateInput
	if len(data) == 0 {
		return input, nil
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return application.EstimateInput{}, err
	}
	return input, nil
}

func decodeEstimateVersionSnapshot(data []byte) (estimateVersionSnapshot, error) {
	var snapshot estimateVersionSnapshot
	if len(data) == 0 {
		return snapshot, nil
	}
	normalized, err := normalizeJSONPropertyNames(data)
	if err != nil {
		return estimateVersionSnapshot{}, err
	}
	if err := json.Unmarshal(normalized, &snapshot); err != nil {
		return estimateVersionSnapshot{}, err
	}
	if snapshot.WorkingInput.Name == "" {
		snapshot.WorkingInput = snapshot.Options.DocumentInput
	}
	return snapshot, nil
}

func compareEstimateVersions(previous, current application.EstimateVersionRecord) ([]estimateVersionChange, error) {
	oldSnapshot, err := decodeEstimateVersionSnapshot(previous.InputSnapshot)
	if err != nil {
		return nil, err
	}
	newSnapshot, err := decodeEstimateVersionSnapshot(current.InputSnapshot)
	if err != nil {
		return nil, err
	}
	changes := make([]estimateVersionChange, 0, 8)
	add := func(name, before, after, reason string) {
		if before != after {
			changes = append(changes, estimateVersionChange{
				Name: name, Before: before, After: after, Reason: reason,
			})
		}
	}
	add("Количество", strconv.Itoa(oldSnapshot.WorkingInput.Quantity), strconv.Itoa(newSnapshot.WorkingInput.Quantity), "")
	oldTime := oldSnapshot.UsedMachineTimeHours * 60
	newTime := newSnapshot.UsedMachineTimeHours * 60
	if math.Abs(oldTime-newTime) >= 0.005 {
		add("Машинное время на деталь", formatMinutes(oldTime), formatMinutes(newTime), newSnapshot.MachineTimeReason)
	}
	add("Наценка", formatPercent(previous.MarkupPercent), formatPercent(current.MarkupPercent), newSnapshot.WorkingInput.MarkupReason)
	add("Оборудование",
		snapshotEquipmentName(previous.ResultSnapshot), snapshotEquipmentName(current.ResultSnapshot), "")
	add("Материал", snapshotMaterialName(previous.SettingsSnapshot), snapshotMaterialName(current.SettingsSnapshot), "")
	add("Заготовка", snapshotStockBlankName(previous.SettingsSnapshot), snapshotStockBlankName(current.SettingsSnapshot), "")

	oldResult, err := decodeEstimateResult(previous.ResultSnapshot)
	if err != nil {
		return nil, err
	}
	newResult, err := decodeEstimateResult(current.ResultSnapshot)
	if err != nil {
		return nil, err
	}
	if oldResult.CostBreakdown != nil && newResult.CostBreakdown != nil {
		add("Себестоимость партии",
			formatMoney(oldResult.CostBreakdown.TotalCost, oldResult.CostBreakdown.Currency),
			formatMoney(newResult.CostBreakdown.TotalCost, newResult.CostBreakdown.Currency), "")
		add("Цена партии",
			formatMoney(oldResult.CostBreakdown.SellingPrice, oldResult.CostBreakdown.Currency),
			formatMoney(newResult.CostBreakdown.SellingPrice, newResult.CostBreakdown.Currency), "")
	}
	return changes, nil
}

func snapshotEquipmentName(data []byte) string {
	var snapshot model.Estimate
	if json.Unmarshal(data, &snapshot) == nil && snapshot.Equipment.Name != "" {
		return snapshot.Equipment.Name
	}
	return "—"
}

func snapshotMaterialName(data []byte) string {
	var snapshot struct {
		Material       *model.Material       `json:"Material"`
		GlobalMaterial *model.GlobalMaterial `json:"GlobalMaterial"`
	}
	if json.Unmarshal(data, &snapshot) != nil {
		return "—"
	}
	if snapshot.Material != nil {
		return snapshot.Material.Name + " " + snapshot.Material.Grade
	}
	if snapshot.GlobalMaterial != nil {
		return snapshot.GlobalMaterial.Name + " " + snapshot.GlobalMaterial.Grade
	}
	return "Не выбран"
}

func snapshotStockBlankName(data []byte) string {
	var snapshot struct {
		StockBlank *model.StockBlank `json:"StockBlank"`
	}
	if json.Unmarshal(data, &snapshot) != nil {
		return "—"
	}
	if snapshot.StockBlank == nil {
		return "Не выбрана"
	}
	return snapshot.StockBlank.Name
}

func decodeEstimateResult(data []byte) (model.Estimate, error) {
	normalized, err := normalizeJSONPropertyNames(data)
	if err != nil {
		return model.Estimate{}, err
	}
	var result model.Estimate
	if err := json.Unmarshal(normalized, &result); err != nil {
		return model.Estimate{}, err
	}
	// Older result snapshots stored the breakdown itself as the root object.
	// Keep displaying the saved values; never rebuild them from current settings.
	if result.CostBreakdown == nil {
		var root map[string]json.RawMessage
		if err := json.Unmarshal(normalized, &root); err != nil {
			return model.Estimate{}, err
		}
		if _, hasMaterialCost := root["material_cost"]; hasMaterialCost {
			var breakdown model.ProductionCostBreakdown
			if err := json.Unmarshal(normalized, &breakdown); err != nil {
				return model.Estimate{}, err
			}
			result.CostBreakdown = &breakdown
		}
	}
	normalizeEstimateResultForDisplay(&result)
	return result, nil
}

func normalizeJSONPropertyNames(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	normalized, err := normalizeJSONValue(value)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func normalizeJSONValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, child := range typed {
			name := normalizeJSONPropertyName(key)
			if _, exists := normalized[name]; exists {
				return nil, fmt.Errorf("duplicate JSON property after normalization: %q", name)
			}
			normalizedChild, err := normalizeJSONValue(child)
			if err != nil {
				return nil, err
			}
			normalized[name] = normalizedChild
		}
		return normalized, nil
	case []any:
		normalized := make([]any, len(typed))
		for index, child := range typed {
			normalizedChild, err := normalizeJSONValue(child)
			if err != nil {
				return nil, err
			}
			normalized[index] = normalizedChild
		}
		return normalized, nil
	default:
		return value, nil
	}
}

func normalizeJSONPropertyName(name string) string {
	var normalized strings.Builder
	characters := []rune(name)
	for index, character := range characters {
		if character >= 'A' && character <= 'Z' {
			previousIsLowerOrDigit := index > 0 &&
				((characters[index-1] >= 'a' && characters[index-1] <= 'z') ||
					(characters[index-1] >= '0' && characters[index-1] <= '9'))
			acronymBoundary := index > 0 && characters[index-1] >= 'A' &&
				characters[index-1] <= 'Z' && index+1 < len(characters) &&
				characters[index+1] >= 'a' && characters[index+1] <= 'z'
			if previousIsLowerOrDigit || acronymBoundary {
				normalized.WriteByte('_')
			}
			character += 'a' - 'A'
		}
		normalized.WriteRune(character)
	}
	return normalized.String()
}

func normalizeEstimateResultForDisplay(result *model.Estimate) {
	if result.CostBreakdown == nil {
		return
	}
	breakdown := result.CostBreakdown
	if breakdown.TotalPerPieceCost == 0 && breakdown.Quantity > 0 {
		breakdown.TotalPerPieceCost = breakdown.TotalCost / float64(breakdown.Quantity)
	}
}

func prepareEstimateCostDisplay(page *lifecyclePage, settingsSnapshot []byte, inputSnapshot ...[]byte) error {
	breakdown := page.Result.CostBreakdown
	if breakdown == nil {
		return nil
	}
	page.CostItems = estimateCostItemsForDisplay(*breakdown)
	page.CostWarnings = localizeEstimateCostWarnings(breakdown.Warnings)
	page.MaterialPriceLabel, page.MaterialPriceNote = estimateMaterialPriceDisplay(breakdown.MaterialPriceSource)
	name, err := estimateMaterialNameFromSnapshot(settingsSnapshot)
	if err != nil {
		return err
	}
	page.MaterialName = name
	var input []byte
	if len(inputSnapshot) > 0 {
		input = inputSnapshot[0]
	}
	page.CostCalculations = explainSavedCosts(page, settingsSnapshot, input)
	page.CostTotals = savedCostTotals(breakdown)
	return nil
}

func estimateMaterialNameFromSnapshot(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	normalized, err := normalizeJSONPropertyNames(data)
	if err != nil {
		return "", err
	}
	var snapshot map[string]any
	if err := json.Unmarshal(normalized, &snapshot); err != nil {
		return "", err
	}
	for _, key := range []string{"material", "global_material"} {
		material, ok := snapshot[key].(map[string]any)
		if !ok {
			continue
		}
		name, _ := material["name"].(string)
		grade, _ := material["grade"].(string)
		name, grade = strings.TrimSpace(name), strings.TrimSpace(grade)
		if name == "" {
			return grade, nil
		}
		if grade != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(grade)) {
			return name + " " + grade, nil
		}
		return name, nil
	}
	return "", nil
}

func estimateCostItemsForDisplay(breakdown model.ProductionCostBreakdown) []model.ProductionCostItem {
	items := make([]model.ProductionCostItem, 0, len(breakdown.CostItems)+14)
	present := make(map[string]bool, len(breakdown.CostItems)+14)
	for _, item := range breakdown.CostItems {
		switch strings.ToLower(strings.TrimSpace(item.Name)) {
		case "себестоимость", "прибыль", "цена продажи":
			continue
		}
		items = append(items, item)
		present[strings.ToLower(strings.TrimSpace(item.Name))] = true
	}

	quantity := float64(breakdown.Quantity)
	perPiece := func(batch float64) float64 {
		if quantity <= 0 {
			return 0
		}
		return batch / quantity
	}
	add := func(name string, batch float64) {
		if present[strings.ToLower(name)] {
			return
		}
		items = append(items, model.ProductionCostItem{
			Name: name, BatchCost: batch, PerPieceCost: perPiece(batch),
		})
		present[strings.ToLower(name)] = true
	}
	add("Материал", breakdown.MaterialCost)
	if breakdown.EquipmentEconomicsApplied {
		add("Амортизация станка", breakdown.DepreciationCost)
		add("Электроэнергия станка", breakdown.ElectricityCost)
		add("Обслуживание станка", breakdown.MaintenanceCost)
		add("Ремонт станка", breakdown.RepairCost)
		add("Прочие расходы станка", breakdown.OtherEquipmentCost)
	} else {
		add("Затраты на станок", breakdown.MachineCost)
	}
	add("Оператор", breakdown.OperatorCost)
	add("Программирование", breakdown.ProgrammingCost)
	add("Инструмент", breakdown.ToolingCost)
	add("Наладка", breakdown.SetupCost)
	add("Аренда", breakdown.RentCost)
	add("Коммунальные расходы", breakdown.UtilitiesCost)
	add("Прочие накладные расходы", breakdown.OtherOverheadCost)
	return items
}

func estimateMaterialPriceDisplay(source string) (string, string) {
	switch source {
	case "global_reference_price":
		return "Ориентировочная цена из общей базы материалов",
			"Цена материала взята из общей базы и является ориентировочной, а не фактической закупочной ценой."
	case "company_material_price_per_kg", "company_stock_blank":
		return "цена из материалов компании", ""
	case "not_selected", "":
		return "не указана", ""
	default:
		return "сохранённый источник цены", ""
	}
}

func localizeEstimateCostWarnings(warnings []string) []string {
	localized := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		switch {
		case strings.HasPrefix(warning, "Machine time manually overridden:"):
			reason := strings.TrimSpace(strings.TrimPrefix(warning, "Machine time manually overridden:"))
			message := "Машинное время указано вручную."
			if reason != "" {
				message += " Причина: " + reason
			}
			localized = append(localized, message)
		case strings.HasPrefix(warning, "Tool ") && strings.Contains(warning, " resource is not configured"):
			localized = append(localized, "Для инструмента не задан ресурс; затраты на инструмент не учитываются.")
		case strings.HasPrefix(warning, "Overhead ") && strings.Contains(warning, " has no production-hour allocation basis"):
			localized = append(localized, "Для прочих накладных расходов не задана база распределения по производственным часам; они не учитываются.")
		case strings.Contains(warning, "Global material price is a reference estimate"):
			localized = append(localized, "Цена материала взята из общей базы и является ориентировочной, а не фактической закупочной ценой.")
		case strings.Contains(warning, "Equipment economics are not configured"):
			localized = append(localized, "Не настроены параметры станка; используется сохранённая базовая почасовая ставка.")
		case strings.Contains(warning, "Legacy machine hourly rate is excluded"):
			localized = append(localized, "Базовая почасовая ставка станка не учитывается, так как настроены подробные параметры станка.")
		case strings.Contains(warning, "Operator rate is not configured"):
			localized = append(localized, "Не настроена стоимость работы оператора; затраты на оператора не учитываются.")
		case strings.Contains(warning, "Programming rate is not configured"):
			localized = append(localized, "Не настроена стоимость программирования; затраты на программирование не учитываются.")
		case strings.Contains(warning, "Setup cost rate is not configured"):
			localized = append(localized, "Не настроена стоимость наладки; затраты на наладку не учитываются.")
		case strings.Contains(warning, "Rent is not configured"):
			localized = append(localized, "Не настроена стоимость аренды; затраты на аренду не учитываются.")
		case strings.Contains(warning, "Material price is not selected"):
			localized = append(localized, "Не выбрана цена материала; стоимость материала не учитывается.")
		case strings.Contains(warning, "Tooling is not selected"):
			localized = append(localized, "Инструмент не выбран; затраты на инструмент не учитываются.")
		case strings.Contains(warning, "no production-hour allocation basis"):
			localized = append(localized, "Для накладных расходов не задана база распределения по производственным часам; они не учитываются.")
		default:
			localized = append(localized, "Требуется проверить параметры расчёта.")
		}
	}
	return localized
}

func formatMinutes(minutes float64) string {
	return strconv.FormatFloat(minutes, 'f', 2, 64) + " мин"
}

func formatPercent(percent float64) string {
	return strconv.FormatFloat(percent, 'f', 2, 64) + "%"
}

func setEstimateTimeDisplay(page *lifecyclePage, data []byte) error {
	snapshot, err := decodeEstimateVersionSnapshot(data)
	if err != nil {
		return err
	}
	page.AutomaticMachineTimeMinutes = snapshot.AutomaticMachineTimeHours * 60
	page.UsedMachineTimeMinutes = snapshot.UsedMachineTimeHours * 60
	page.MachineTimeSource, page.MachineTimeReason = snapshot.MachineTimeSource, snapshot.MachineTimeReason
	page.HasAutomaticMachineTime = true
	return nil
}

func (s *Server) renderEstimateEditError(w http.ResponseWriter, r *http.Request, estimateID string, err error) {
	status := http.StatusInternalServerError
	message := "Не удалось пересчитать смету. Повторите попытку."
	if errors.Is(err, application.ErrNotFound) {
		status, message = http.StatusNotFound, "Смета не найдена или недоступна в текущей компании."
	} else if isLifecycleValidationError(err) {
		status, message = http.StatusBadRequest, lifecyclePublicError(err)
	} else {
		log.Printf("calculate estimate %s: %v", estimateID, err)
	}
	document, getErr := s.lifecycle.GetEstimateDocument(r.Context(), companyIDForRequest(s, r), estimateID)
	if getErr != nil {
		s.lifecycleError(w, r, "reload estimate after calculation error", getErr)
		return
	}
	var input application.EstimateInput
	if err := json.Unmarshal(document.Estimate.WorkingInput, &input); err != nil {
		s.writeInternalError(w, "decode estimate input while rendering calculation error", err)
		return
	}
	var snapshot estimateVersionSnapshot
	if input.Name == "" && len(document.Versions) > 0 {
		snapshot, err = decodeEstimateVersionSnapshot(document.Versions[0].InputSnapshot)
		if err != nil {
			s.writeInternalError(w, "decode previous estimate input while rendering error", err)
			return
		}
		input = snapshot.WorkingInput
	}
	equipment, equipmentErr := s.lifecycle.ListEquipment(r.Context(), companyIDForRequest(s, r))
	materials, materialsErr := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	globalMaterials, globalErr := s.lifecycle.ListGlobalMaterials(r.Context())
	stockBlanks, stockErr := s.lifecycle.ListStockBlanks(r.Context(), companyIDForRequest(s, r))
	consumables, consumablesErr := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
	orders, ordersErr := s.lifecycle.ListOrders(r.Context(), companyIDForRequest(s, r))
	for _, loadErr := range []error{equipmentErr, materialsErr, globalErr, stockErr, consumablesErr, ordersErr} {
		if loadErr != nil {
			s.lifecycleError(w, r, "reload estimate form data", loadErr)
			return
		}
	}
	page := s.lifecycleBase(r, document.Estimate.Number+" — редактирование", "estimates", "estimate-edit")
	page.Error, page.EstimateInput = message, input
	page.EstimateFormValues = estimateRequestFormValues(r)
	page.Estimate, page.Project, page.Part, page.Revision = document.Estimate, document.Project, document.Part, document.Revision
	if len(document.Versions) > 0 {
		if snapshot.WorkingInput.Name == "" {
			snapshot, err = decodeEstimateVersionSnapshot(document.Versions[0].InputSnapshot)
			if err != nil {
				s.writeInternalError(w, "decode latest estimate snapshot while rendering error", err)
				return
			}
		}
		page.AutomaticMachineTimeMinutes = snapshot.AutomaticMachineTimeHours * 60
		page.UsedMachineTimeMinutes = snapshot.UsedMachineTimeHours * 60
		page.HasAutomaticMachineTime = true
	}
	page.Equipment, page.Materials, page.GlobalMaterials = activeEquipment(equipment), materials, globalMaterials
	page.StockBlanks, page.Consumables, page.Orders = stockBlanks, consumables, orders
	if !s.loadEstimateGlobalTools(w, r, &page) {
		return
	}
	s.render(w, r, "lifecycle.html", status, page)
}

func (s *Server) renderEstimateVersion(w http.ResponseWriter, r *http.Request, estimateID, versionID string) {
	s.setCSRFCookie(w, r)
	detail, err := s.lifecycle.GetEstimateVersion(r.Context(), companyIDForRequest(s, r), estimateID, versionID)
	if err != nil {
		s.lifecycleError(w, r, "get estimate version", err)
		return
	}
	result, err := decodeEstimateResult(detail.Version.ResultSnapshot)
	if err != nil {
		s.writeInternalError(w, "decode saved estimate result", err)
		return
	}
	snapshot, err := decodeEstimateVersionSnapshot(detail.Version.InputSnapshot)
	if err != nil {
		s.writeInternalError(w, "decode saved estimate input metadata", err)
		return
	}
	document, err := s.lifecycle.GetEstimateDocument(r.Context(), companyIDForRequest(s, r), estimateID)
	if err != nil {
		s.lifecycleError(w, r, "load estimate versions for comparison", err)
		return
	}
	page := s.lifecycleBase(r, detail.Estimate.Number+" — "+detail.Estimate.Name, "estimates", "version")
	page.Estimate, page.Version, page.Result = detail.Estimate, detail.Version, result
	if err := prepareEstimateCostDisplay(&page, detail.Version.SettingsSnapshot, detail.Version.InputSnapshot); err != nil {
		s.writeInternalError(w, "decode saved estimate material snapshot", err)
		return
	}
	page.Currency = detail.Version.Currency
	page.Project, page.Part, page.Revision = document.Project, document.Part, document.Revision
	page.Versions = document.Versions
	page.IsCurrentVersion = detail.Version.ID == document.Estimate.CurrentVersionID
	page.AutomaticMachineTimeMinutes = snapshot.AutomaticMachineTimeHours * 60
	page.UsedMachineTimeMinutes = snapshot.UsedMachineTimeHours * 60
	page.MachineTimeSource, page.MachineTimeReason = snapshot.MachineTimeSource, snapshot.MachineTimeReason
	page.HasAutomaticMachineTime = true
	for _, previous := range document.Versions {
		if previous.VersionNumber == detail.Version.VersionNumber-1 {
			page.HasPreviousVersion = true
			page.PreviousVersionNumber = previous.VersionNumber
			page.VersionChanges, err = compareEstimateVersions(previous, detail.Version)
			if err != nil {
				s.writeInternalError(w, "compare estimate versions", err)
				return
			}
			break
		}
	}
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
	materials, getErr := s.lifecycle.ListMaterials(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load materials after validation error", getErr)
		return
	}
	globalMaterials, getErr := s.lifecycle.ListGlobalMaterials(r.Context())
	if getErr != nil {
		s.lifecycleError(w, r, "load global materials after validation error", getErr)
		return
	}
	stockBlanks, getErr := s.lifecycle.ListStockBlanks(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load stock blanks after validation error", getErr)
		return
	}
	consumables, getErr := s.lifecycle.ListConsumables(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load tooling after validation error", getErr)
		return
	}
	orders, getErr := s.lifecycle.ListOrders(r.Context(), companyIDForRequest(s, r))
	if getErr != nil {
		s.lifecycleError(w, r, "load orders after validation error", getErr)
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
	page.Materials, page.GlobalMaterials = materials, globalMaterials
	page.StockBlanks, page.Consumables, page.Orders = stockBlanks, consumables, orders
	if !s.loadEstimateGlobalTools(w, r, &page) {
		return
	}
	page.Markup = formatNumber(settings.DefaultMarkupPercent)
	page.Currency = settings.Currency
	page.EstimateFormValues = createEstimateFormValues(page.Markup)
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/revisions/") &&
		strings.HasSuffix(r.URL.Path, "/estimates") {
		page.EstimateFormValues = estimateRequestFormValues(r)
	}
	page.Error = lifecyclePublicError(err)
	status := http.StatusBadRequest
	if errors.Is(err, application.ErrNotFound) {
		status = http.StatusNotFound
	} else if !isLifecycleValidationError(err) {
		status = http.StatusInternalServerError
	}

	s.render(w, r, "lifecycle.html", status, page)
}

func (s *Server) loadEstimateGlobalTools(w http.ResponseWriter, r *http.Request, page *lifecyclePage) bool {
	tools, err := s.lifecycle.ListGlobalTools(r.Context())
	if err != nil {
		s.writeInternalError(w, "load global tools", err)
		return false
	}
	page.GlobalTools = tools
	return true
}

func createEstimateFormValues(markup string) map[string]string {
	return map[string]string{
		"name": "Расчёт", "order_id": "", "quantity": "1", "equipment_id": "",
		"markup_percent": markup, "markup_source": string(application.EstimateSourceCompanyData),
		"markup_reason": "", "setup_time_minutes": "0", "setup_reason": "",
		"programming_hours": "0", "programming_reason": "", "tool_usage_minutes": "0",
		"tool_usage_reason": "",
	}
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

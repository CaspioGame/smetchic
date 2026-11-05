package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"smetchic/application"
	"smetchic/model"
)

type LifecycleRepository struct {
	db *DB
}

func NewLifecycleRepository(db *DB) *LifecycleRepository {
	return &LifecycleRepository{db: db}
}

func (repository *LifecycleRepository) EnsureCompany(ctx context.Context, company model.Company, settings model.CompanySettings) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company bootstrap: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO companies (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING`, company.ID, company.Name)
	if err != nil {
		return fmt.Errorf("ensure company: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO company_settings (company_id, currency, default_markup_percent)
		VALUES ($1, $2, $3)
		ON CONFLICT (company_id) DO NOTHING`,
		settings.CompanyID, settings.Currency, settings.DefaultMarkupPercent)
	if err != nil {
		return fmt.Errorf("ensure company settings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company bootstrap: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) CreateProject(ctx context.Context, project application.ProjectRecord) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO projects (id, company_id, name, description)
		VALUES ($1, $2, $3, $4)`, project.ID, project.CompanyID, project.Name, project.Description)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) GetProject(ctx context.Context, companyID, projectID string) (application.ProjectRecord, error) {
	var project application.ProjectRecord
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, company_id, name, description, created_at, updated_at
		FROM projects WHERE company_id = $1 AND id = $2`, companyID, projectID).
		Scan(&project.ID, &project.CompanyID, &project.Name, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return application.ProjectRecord{}, notFound("get project", err)
	}
	return project, nil
}

func (repository *LifecycleRepository) ListProjects(ctx context.Context, companyID string) ([]application.ProjectRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, name, description, created_at, updated_at
		FROM projects WHERE company_id = $1 ORDER BY updated_at DESC, id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	projects := make([]application.ProjectRecord, 0)
	for rows.Next() {
		var project application.ProjectRecord
		if err := rows.Scan(&project.ID, &project.CompanyID, &project.Name, &project.Description, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

func (repository *LifecycleRepository) ListParts(ctx context.Context, companyID, projectID string) ([]application.PartRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, project_id, name, description, created_at, updated_at
		FROM parts WHERE company_id = $1 AND project_id = $2 ORDER BY created_at, id`, companyID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list parts: %w", err)
	}
	defer rows.Close()
	parts := make([]application.PartRecord, 0)
	for rows.Next() {
		var part application.PartRecord
		if err := rows.Scan(&part.ID, &part.CompanyID, &part.ProjectID, &part.Name, &part.Description, &part.CreatedAt, &part.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan part: %w", err)
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parts: %w", err)
	}
	return parts, nil
}

func (repository *LifecycleRepository) GetPart(ctx context.Context, companyID, partID string) (application.PartRecord, error) {
	var part application.PartRecord
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, company_id, project_id, name, description, created_at, updated_at
		FROM parts WHERE company_id = $1 AND id = $2`, companyID, partID).
		Scan(&part.ID, &part.CompanyID, &part.ProjectID, &part.Name, &part.Description, &part.CreatedAt, &part.UpdatedAt)
	if err != nil {
		return application.PartRecord{}, notFound("get part", err)
	}
	return part, nil
}

func (repository *LifecycleRepository) CreatePartWithRevision(ctx context.Context, part application.PartRecord, revision application.PartRevisionRecord) (application.PartRecord, application.PartRevisionRecord, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("begin part revision transaction: %w", err)
	}
	defer tx.Rollback()

	var projectID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM projects WHERE company_id = $1 AND id = $2 FOR UPDATE`,
		part.CompanyID, part.ProjectID).Scan(&projectID); err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, notFound("find project for part", err)
	}
	err = tx.QueryRowContext(ctx, `
		SELECT id, company_id, project_id, name, description, created_at, updated_at
		FROM parts WHERE company_id = $1 AND project_id = $2 AND name = $3`,
		part.CompanyID, part.ProjectID, part.Name).
		Scan(&part.ID, &part.CompanyID, &part.ProjectID, &part.Name, &part.Description, &part.CreatedAt, &part.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO parts (id, company_id, project_id, name, description)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at, updated_at`,
			part.ID, part.CompanyID, part.ProjectID, part.Name, part.Description).
			Scan(&part.CreatedAt, &part.UpdatedAt)
	}
	if err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("find or create part: %w", err)
	}

	revision.PartID = part.ID
	revision.CompanyID = part.CompanyID
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(revision_number), 0) + 1
		FROM part_revisions WHERE company_id = $1 AND part_id = $2`,
		part.CompanyID, part.ID).Scan(&revision.RevisionNumber)
	if err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("allocate revision number: %w", err)
	}
	parsed, err := json.Marshal(revision.ParsedPart)
	if err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("marshal parsed part: %w", err)
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO part_revisions (
			id, company_id, part_id, revision_number, source_format, source_filename,
			source_content, source_hash, parser_version, parsed_part_json
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`,
		revision.ID, revision.CompanyID, revision.PartID, revision.RevisionNumber,
		revision.SourceFormat, revision.SourceFilename, revision.SourceContent,
		revision.SourceHash, revision.ParserVersion, parsed).Scan(&revision.CreatedAt)
	if err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("save part revision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return application.PartRecord{}, application.PartRevisionRecord{}, fmt.Errorf("commit part revision: %w", err)
	}
	return part, revision, nil
}

func (repository *LifecycleRepository) GetRevisionEstimate(ctx context.Context, companyID, revisionID string) (application.RevisionEstimate, error) {
	var detail application.RevisionEstimate
	var parsed []byte
	err := repository.db.QueryRowContext(ctx, `
		SELECT
			projects.id, projects.company_id, projects.name, projects.description, projects.created_at, projects.updated_at,
			parts.id, parts.company_id, parts.project_id, parts.name, parts.description, parts.created_at, parts.updated_at,
			revisions.id, revisions.company_id, revisions.part_id, revisions.revision_number,
			revisions.source_format, revisions.source_filename, revisions.source_content,
			revisions.source_hash, revisions.parser_version, revisions.parsed_part_json, revisions.created_at
		FROM part_revisions AS revisions
		JOIN parts ON parts.company_id = revisions.company_id AND parts.id = revisions.part_id
		JOIN projects ON projects.company_id = parts.company_id AND projects.id = parts.project_id
		WHERE revisions.company_id = $1 AND revisions.id = $2`,
		companyID, revisionID).Scan(
		&detail.Project.ID, &detail.Project.CompanyID, &detail.Project.Name, &detail.Project.Description, &detail.Project.CreatedAt, &detail.Project.UpdatedAt,
		&detail.Part.ID, &detail.Part.CompanyID, &detail.Part.ProjectID, &detail.Part.Name, &detail.Part.Description, &detail.Part.CreatedAt, &detail.Part.UpdatedAt,
		&detail.Revision.ID, &detail.Revision.CompanyID, &detail.Revision.PartID, &detail.Revision.RevisionNumber,
		&detail.Revision.SourceFormat, &detail.Revision.SourceFilename, &detail.Revision.SourceContent,
		&detail.Revision.SourceHash, &detail.Revision.ParserVersion, &parsed, &detail.Revision.CreatedAt)
	if err != nil {
		return application.RevisionEstimate{}, notFound("get part revision", err)
	}
	if err := json.Unmarshal(parsed, &detail.Revision.ParsedPart); err != nil {
		return application.RevisionEstimate{}, fmt.Errorf("decode parsed part: %w", err)
	}
	detail.Estimates, err = repository.ListEstimatesForRevision(ctx, companyID, revisionID)
	if err != nil {
		return application.RevisionEstimate{}, err
	}
	return detail, nil
}

func (repository *LifecycleRepository) ListPartRevisions(ctx context.Context, companyID, partID string) ([]application.PartRevisionRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, part_id, revision_number, source_format, source_filename,
			source_content, source_hash, parser_version, parsed_part_json, created_at
		FROM part_revisions WHERE company_id = $1 AND part_id = $2
		ORDER BY revision_number DESC`, companyID, partID)
	if err != nil {
		return nil, fmt.Errorf("list part revisions: %w", err)
	}
	defer rows.Close()
	revisions := make([]application.PartRevisionRecord, 0)
	for rows.Next() {
		var revision application.PartRevisionRecord
		var parsed []byte
		if err := rows.Scan(
			&revision.ID, &revision.CompanyID, &revision.PartID, &revision.RevisionNumber,
			&revision.SourceFormat, &revision.SourceFilename, &revision.SourceContent,
			&revision.SourceHash, &revision.ParserVersion, &parsed, &revision.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan part revision: %w", err)
		}
		if err := json.Unmarshal(parsed, &revision.ParsedPart); err != nil {
			return nil, fmt.Errorf("decode part revision: %w", err)
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate part revisions: %w", err)
	}
	return revisions, nil
}

func (repository *LifecycleRepository) ListEstimatesForRevision(ctx context.Context, companyID, revisionID string) ([]application.EstimateRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, part_revision_id, name, created_at, updated_at
		FROM estimates WHERE company_id = $1 AND part_revision_id = $2
		ORDER BY updated_at DESC, id`, companyID, revisionID)
	if err != nil {
		return nil, fmt.Errorf("list estimates for revision: %w", err)
	}
	defer rows.Close()
	estimates := make([]application.EstimateRecord, 0)
	for rows.Next() {
		var estimate application.EstimateRecord
		if err := rows.Scan(&estimate.ID, &estimate.CompanyID, &estimate.PartRevisionID, &estimate.Name, &estimate.CreatedAt, &estimate.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan estimate: %w", err)
		}
		estimates = append(estimates, estimate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate estimates: %w", err)
	}
	return estimates, nil
}

func (repository *LifecycleRepository) CreateEstimateVersion(ctx context.Context, estimate application.EstimateRecord, version application.EstimateVersionRecord) (application.EstimateRecord, application.EstimateVersionRecord, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("begin estimate version transaction: %w", err)
	}
	defer tx.Rollback()

	var revisionID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM part_revisions
		WHERE company_id = $1 AND id = $2 FOR UPDATE`,
		estimate.CompanyID, estimate.PartRevisionID).Scan(&revisionID); err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, notFound("find estimate revision", err)
	}
	err = tx.QueryRowContext(ctx, `
		SELECT id, company_id, part_revision_id, name, created_at, updated_at
		FROM estimates
		WHERE company_id = $1 AND part_revision_id = $2 AND name = $3
		FOR UPDATE`,
		estimate.CompanyID, estimate.PartRevisionID, estimate.Name).
		Scan(&estimate.ID, &estimate.CompanyID, &estimate.PartRevisionID, &estimate.Name, &estimate.CreatedAt, &estimate.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO estimates (id, company_id, part_revision_id, name)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at, updated_at`,
			estimate.ID, estimate.CompanyID, estimate.PartRevisionID, estimate.Name).
			Scan(&estimate.CreatedAt, &estimate.UpdatedAt)
	}
	if err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("find or create estimate: %w", err)
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM estimate_versions WHERE company_id = $1 AND estimate_id = $2`,
		estimate.CompanyID, estimate.ID).Scan(&version.VersionNumber)
	if err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("allocate estimate version: %w", err)
	}
	version.CompanyID = estimate.CompanyID
	version.EstimateID = estimate.ID
	version.PartRevisionID = estimate.PartRevisionID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO estimate_versions (
			id, company_id, estimate_id, part_revision_id, version_number,
			equipment_id, equipment_model_id, input_snapshot, settings_snapshot, result_snapshot,
			machine_time_hours, hourly_rate, machine_cost, markup_percent, final_price, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at`,
		version.ID, version.CompanyID, version.EstimateID, version.PartRevisionID, version.VersionNumber,
		version.EquipmentID, version.EquipmentModelID, json.RawMessage(version.InputSnapshot),
		json.RawMessage(version.SettingsSnapshot), json.RawMessage(version.ResultSnapshot),
		version.MachineTimeHours, version.HourlyRate, version.MachineCost, version.MarkupPercent,
		version.FinalPrice, version.Currency).Scan(&version.CreatedAt)
	if err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("save estimate version: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
		UPDATE estimates SET updated_at = now()
		WHERE company_id = $1 AND id = $2
		RETURNING updated_at`, estimate.CompanyID, estimate.ID).Scan(&estimate.UpdatedAt); err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("update estimate timestamp: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return application.EstimateRecord{}, application.EstimateVersionRecord{}, fmt.Errorf("commit estimate version: %w", err)
	}
	return estimate, version, nil
}

func (repository *LifecycleRepository) GetEstimate(ctx context.Context, companyID, estimateID string) (application.EstimateRecord, error) {
	var estimate application.EstimateRecord
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, company_id, part_revision_id, name, created_at, updated_at
		FROM estimates WHERE company_id = $1 AND id = $2`, companyID, estimateID).
		Scan(&estimate.ID, &estimate.CompanyID, &estimate.PartRevisionID, &estimate.Name, &estimate.CreatedAt, &estimate.UpdatedAt)
	if err != nil {
		return application.EstimateRecord{}, notFound("get estimate", err)
	}
	return estimate, nil
}

func (repository *LifecycleRepository) GetEstimateVersion(ctx context.Context, companyID, estimateID, versionID string) (application.EstimateVersionDetail, error) {
	detail := application.EstimateVersionDetail{}
	var inputSnapshot, settingsSnapshot, resultSnapshot []byte
	err := repository.db.QueryRowContext(ctx, `
		SELECT
			estimates.id, estimates.company_id, estimates.part_revision_id, estimates.name, estimates.created_at, estimates.updated_at,
			versions.id, versions.company_id, versions.estimate_id, versions.part_revision_id, versions.version_number,
			versions.equipment_id, versions.equipment_model_id, versions.input_snapshot, versions.settings_snapshot, versions.result_snapshot,
			versions.machine_time_hours, versions.hourly_rate, versions.machine_cost,
			versions.markup_percent, versions.final_price, versions.currency, versions.created_at
		FROM estimate_versions AS versions
		JOIN estimates ON estimates.company_id = versions.company_id AND estimates.id = versions.estimate_id
		WHERE versions.company_id = $1 AND versions.estimate_id = $2 AND versions.id = $3`,
		companyID, estimateID, versionID).Scan(
		&detail.Estimate.ID, &detail.Estimate.CompanyID, &detail.Estimate.PartRevisionID, &detail.Estimate.Name, &detail.Estimate.CreatedAt, &detail.Estimate.UpdatedAt,
		&detail.Version.ID, &detail.Version.CompanyID, &detail.Version.EstimateID, &detail.Version.PartRevisionID, &detail.Version.VersionNumber,
		&detail.Version.EquipmentID, &detail.Version.EquipmentModelID, &inputSnapshot, &settingsSnapshot, &resultSnapshot,
		&detail.Version.MachineTimeHours, &detail.Version.HourlyRate, &detail.Version.MachineCost,
		&detail.Version.MarkupPercent, &detail.Version.FinalPrice, &detail.Version.Currency, &detail.Version.CreatedAt)
	if err != nil {
		return application.EstimateVersionDetail{}, notFound("get estimate version", err)
	}
	detail.Version.InputSnapshot = inputSnapshot
	detail.Version.SettingsSnapshot = settingsSnapshot
	detail.Version.ResultSnapshot = resultSnapshot
	return detail, nil
}

func (repository *LifecycleRepository) ListEstimateVersions(ctx context.Context, companyID, estimateID string) ([]application.EstimateVersionRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, estimate_id, part_revision_id, version_number,
			equipment_id, equipment_model_id, input_snapshot, settings_snapshot, result_snapshot,
			machine_time_hours, hourly_rate, machine_cost, markup_percent, final_price, currency, created_at
		FROM estimate_versions WHERE company_id = $1 AND estimate_id = $2
		ORDER BY version_number DESC`, companyID, estimateID)
	if err != nil {
		return nil, fmt.Errorf("list estimate versions: %w", err)
	}
	defer rows.Close()
	versions := make([]application.EstimateVersionRecord, 0)
	for rows.Next() {
		var version application.EstimateVersionRecord
		var inputSnapshot, settingsSnapshot, resultSnapshot []byte
		if err := rows.Scan(
			&version.ID, &version.CompanyID, &version.EstimateID, &version.PartRevisionID, &version.VersionNumber,
			&version.EquipmentID, &version.EquipmentModelID, &inputSnapshot, &settingsSnapshot, &resultSnapshot,
			&version.MachineTimeHours, &version.HourlyRate, &version.MachineCost,
			&version.MarkupPercent, &version.FinalPrice, &version.Currency, &version.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan estimate version: %w", err)
		}
		version.InputSnapshot = inputSnapshot
		version.SettingsSnapshot = settingsSnapshot
		version.ResultSnapshot = resultSnapshot
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate estimate versions: %w", err)
	}
	return versions, nil
}

func notFound(operation string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, application.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

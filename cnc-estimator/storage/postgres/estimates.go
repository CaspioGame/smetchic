package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"smetchic/application"
)

func (repository *LifecycleRepository) CreateEstimate(ctx context.Context, estimate application.EstimateRecord) (application.EstimateRecord, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return application.EstimateRecord{}, fmt.Errorf("begin create estimate: %w", err)
	}
	defer tx.Rollback()

	var partID, projectID string
	if err := tx.QueryRowContext(ctx, `
		SELECT parts.id, parts.project_id
		FROM part_revisions
		JOIN parts ON parts.company_id = part_revisions.company_id AND parts.id = part_revisions.part_id
		WHERE part_revisions.company_id = $1 AND part_revisions.id = $2`,
		estimate.CompanyID, estimate.PartRevisionID).Scan(&partID, &projectID); err != nil {
		return application.EstimateRecord{}, notFound("find estimate part revision", err)
	}
	if estimate.ProjectID != projectID || (estimate.PartID != "" && estimate.PartID != partID) {
		return application.EstimateRecord{}, application.Invalid("Деталь и проект сметы не совпадают.", nil)
	}
	if estimate.OrderID != "" {
		var orderProjectID sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT project_id FROM orders WHERE company_id = $1 AND id = $2`,
			estimate.CompanyID, estimate.OrderID).Scan(&orderProjectID); err != nil {
			return application.EstimateRecord{}, notFound("find estimate order", err)
		}
		if orderProjectID.Valid && orderProjectID.String != projectID {
			return application.EstimateRecord{}, application.Invalid("Заказ связан с другим проектом.", nil)
		}
	}
	number, err := allocateEstimateNumber(ctx, tx, estimate.CompanyID)
	if err != nil {
		return application.EstimateRecord{}, err
	}
	if len(estimate.WorkingInput) == 0 {
		estimate.WorkingInput = json.RawMessage(`{}`)
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO estimates (
			id, company_id, part_revision_id, name, number, project_id, part_id,
			order_id, quantity, status, working_input
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, 'draft', $10::jsonb)
		RETURNING created_at, updated_at`,
		estimate.ID, estimate.CompanyID, estimate.PartRevisionID, strings.TrimSpace(estimate.Name),
		number, projectID, partID, estimate.OrderID, estimate.Quantity, string(estimate.WorkingInput)).
		Scan(&estimate.CreatedAt, &estimate.UpdatedAt)
	if err != nil {
		return application.EstimateRecord{}, fmt.Errorf("insert working estimate: %w", err)
	}
	estimate.Number, estimate.ProjectID, estimate.PartID = number, projectID, partID
	estimate.Status = application.EstimateStatusDraft
	if err := tx.Commit(); err != nil {
		return application.EstimateRecord{}, fmt.Errorf("commit create estimate: %w", err)
	}
	return estimate, nil
}

func (repository *LifecycleRepository) UpdateEstimateInput(ctx context.Context, companyID string, estimate application.EstimateRecord) (application.EstimateRecord, error) {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE estimates
		SET name = $3, order_id = NULLIF($4, ''), quantity = $5,
		    working_input = $6::jsonb, status = 'draft', updated_at = now()
		WHERE company_id = $1 AND id = $2`,
		companyID, estimate.ID, strings.TrimSpace(estimate.Name), estimate.OrderID,
		estimate.Quantity, string(estimate.WorkingInput))
	if err != nil {
		return application.EstimateRecord{}, fmt.Errorf("update working estimate input: %w", err)
	}
	if err := requireAffected(result, "update working estimate input"); err != nil {
		return application.EstimateRecord{}, err
	}
	return repository.GetEstimate(ctx, companyID, estimate.ID)
}

func (repository *LifecycleRepository) ListCompanyEstimates(ctx context.Context, companyID string) ([]application.EstimateRecord, error) {
	rows, err := repository.db.QueryContext(ctx, estimateSelect+`
		WHERE estimates.company_id = $1
		ORDER BY estimates.updated_at DESC, estimates.id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company estimates: %w", err)
	}
	defer rows.Close()
	items := make([]application.EstimateRecord, 0)
	for rows.Next() {
		item, err := scanEstimate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan company estimate: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company estimates: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) GetEstimateDocument(ctx context.Context, companyID, estimateID string) (application.EstimateDocument, error) {
	estimate, err := repository.GetEstimate(ctx, companyID, estimateID)
	if err != nil {
		return application.EstimateDocument{}, err
	}
	revision, err := repository.GetRevisionEstimate(ctx, companyID, estimate.PartRevisionID)
	if err != nil {
		return application.EstimateDocument{}, err
	}
	document := application.EstimateDocument{
		Estimate: estimate, Project: revision.Project, Part: revision.Part, Revision: revision.Revision,
	}
	document.Versions, err = repository.ListEstimateVersions(ctx, companyID, estimateID)
	if err != nil {
		return application.EstimateDocument{}, err
	}
	if estimate.OrderID != "" {
		document.Order, err = repository.GetOrder(ctx, companyID, estimate.OrderID)
		if err != nil {
			return application.EstimateDocument{}, err
		}
	}
	return document, nil
}

const estimateSelect = `
	SELECT estimates.id, estimates.company_id, estimates.part_id, estimates.project_id,
	       projects.name,
	       COALESCE(estimates.order_id, ''), estimates.part_revision_id,
	       estimates.number, estimates.name, estimates.quantity, estimates.status,
	       estimates.working_input, COALESCE(estimates.current_version_id, ''),
	       estimates.created_at, estimates.updated_at
	FROM estimates
	JOIN projects ON projects.company_id = estimates.company_id AND projects.id = estimates.project_id`

type estimateScanner interface {
	Scan(...any) error
}

func scanEstimate(scanner estimateScanner) (application.EstimateRecord, error) {
	var estimate application.EstimateRecord
	var workingInput []byte
	err := scanner.Scan(
		&estimate.ID, &estimate.CompanyID, &estimate.PartID, &estimate.ProjectID,
		&estimate.ProjectName, &estimate.OrderID, &estimate.PartRevisionID, &estimate.Number, &estimate.Name,
		&estimate.Quantity, &estimate.Status, &workingInput, &estimate.CurrentVersionID,
		&estimate.CreatedAt, &estimate.UpdatedAt,
	)
	if err != nil {
		return application.EstimateRecord{}, err
	}
	estimate.WorkingInput = append(estimate.WorkingInput[:0], workingInput...)
	return estimate, nil
}

func allocateEstimateNumber(ctx context.Context, tx *sql.Tx, companyID string) (string, error) {
	var next int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO estimate_number_counters (company_id, current_number)
		VALUES ($1, 1)
		ON CONFLICT (company_id) DO UPDATE
		SET current_number = estimate_number_counters.current_number + 1
		RETURNING current_number`, companyID).Scan(&next); err != nil {
		return "", fmt.Errorf("allocate estimate number: %w", err)
	}
	return fmt.Sprintf("СМ-%06d", next), nil
}

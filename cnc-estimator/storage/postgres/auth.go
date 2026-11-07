package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"smetchic/application"
	"smetchic/model"
)

func (repository *LifecycleRepository) GetAuthUserByEmail(ctx context.Context, email string) (application.AuthUserRecord, error) {
	record, err := scanAuthUser(repository.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, is_active, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)`, email))
	if err != nil {
		return application.AuthUserRecord{}, notFound("get user by email", err)
	}
	return record, nil
}

func (repository *LifecycleRepository) GetAuthUserByID(ctx context.Context, userID string) (application.AuthUserRecord, error) {
	record, err := scanAuthUser(repository.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, is_active, created_at, updated_at
		FROM users WHERE id = $1`, userID))
	if err != nil {
		return application.AuthUserRecord{}, notFound("get user by ID", err)
	}
	return record, nil
}

func (repository *LifecycleRepository) ListUserMemberships(ctx context.Context, userID string) ([]application.AuthMembership, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT companies.id, companies.name, companies.created_at, companies.updated_at,
		       memberships.role
		FROM company_memberships AS memberships
		JOIN companies ON companies.id = memberships.company_id
		WHERE memberships.user_id = $1
		ORDER BY companies.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user memberships: %w", err)
	}
	defer rows.Close()
	memberships := make([]application.AuthMembership, 0)
	for rows.Next() {
		var membership application.AuthMembership
		if err := rows.Scan(
			&membership.Company.ID, &membership.Company.Name,
			&membership.Company.CreatedAt, &membership.Company.UpdatedAt,
			&membership.Role,
		); err != nil {
			return nil, fmt.Errorf("scan user membership: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user memberships: %w", err)
	}
	return memberships, nil
}

func (repository *LifecycleRepository) CreateUserWithMembership(
	ctx context.Context,
	user model.User,
	passwordHash string,
	companyID string,
	role model.CompanyRole,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create user transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, password_hash)
		VALUES ($1, $2, $3, $4)`, user.ID, user.Email, user.Name, passwordHash)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO company_memberships (company_id, user_id, role)
		VALUES ($1, $2, $3)`, companyID, user.ID, role)
	if err != nil {
		return fmt.Errorf("insert user company membership: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create user transaction: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) RegisterCompanyWithOwner(
	ctx context.Context,
	company model.Company,
	settings model.CompanySettings,
	user model.User,
	passwordHash string,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company registration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO companies (id, name) VALUES ($1, $2)`,
		company.ID, company.Name); err != nil {
		return fmt.Errorf("insert registered company: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_settings (company_id, currency, default_markup_percent)
		VALUES ($1, $2, $3)`,
		settings.CompanyID, settings.Currency, settings.DefaultMarkupPercent); err != nil {
		return fmt.Errorf("insert registered company settings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_economics (company_id, currency, default_markup_percent)
		VALUES ($1, $2, $3)`,
		settings.CompanyID, settings.Currency, settings.DefaultMarkupPercent); err != nil {
		return fmt.Errorf("insert registered company production economics: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, name, password_hash)
		VALUES ($1, $2, $3, $4)`,
		user.ID, user.Email, user.Name, passwordHash); err != nil {
		if isUniqueConstraint(err, "users_email_lower_uq") {
			return application.ErrEmailAlreadyExists
		}
		return fmt.Errorf("insert registered user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_memberships (company_id, user_id, role)
		VALUES ($1, $2, $3)`,
		company.ID, user.ID, model.CompanyRoleOwner); err != nil {
		return fmt.Errorf("insert registration membership: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company registration: %w", err)
	}
	return nil
}

func isUniqueConstraint(err error, constraint string) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" && postgresError.ConstraintName == constraint
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAuthUser(row rowScanner) (application.AuthUserRecord, error) {
	var record application.AuthUserRecord
	err := row.Scan(
		&record.User.ID, &record.User.Email, &record.User.Name,
		&record.PasswordHash, &record.Active,
		&record.User.CreatedAt, &record.User.UpdatedAt,
	)
	if err != nil {
		return application.AuthUserRecord{}, err
	}
	return record, nil
}

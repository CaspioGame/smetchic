package postgres

import (
	"context"
	"fmt"

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

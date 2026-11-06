package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"smetchic/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user is inactive")
)

const (
	bcryptCost      = 12
	minimumPassword = 12
	maximumPassword = 72
)

type AuthUserRecord struct {
	User         model.User
	PasswordHash string
	Active       bool
}

type AuthMembership struct {
	Company model.Company
	Role    model.CompanyRole
}

type AuthRepository interface {
	GetAuthUserByEmail(context.Context, string) (AuthUserRecord, error)
	GetAuthUserByID(context.Context, string) (AuthUserRecord, error)
	ListUserMemberships(context.Context, string) ([]AuthMembership, error)
	CreateUserWithMembership(context.Context, model.User, string, string, model.CompanyRole) error
}

type AuthenticationService struct {
	repository AuthRepository
}

func NewAuthenticationService(repository AuthRepository) *AuthenticationService {
	return &AuthenticationService{repository: repository}
}

func (service *AuthenticationService) Login(ctx context.Context, email, password string) (model.User, error) {
	record, err := service.repository.GetAuthUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return model.User{}, ErrInvalidCredentials
		}
		return model.User{}, fmt.Errorf("load user for login: %w", err)
	}
	if !record.Active || bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)) != nil {
		return model.User{}, ErrInvalidCredentials
	}
	return record.User, nil
}

func (service *AuthenticationService) ResolveUser(ctx context.Context, userID string) (AuthUserRecord, []AuthMembership, error) {
	record, err := service.repository.GetAuthUserByID(ctx, userID)
	if err != nil {
		return AuthUserRecord{}, nil, fmt.Errorf("load authenticated user: %w", err)
	}
	if !record.Active {
		return AuthUserRecord{}, nil, ErrInactiveUser
	}
	memberships, err := service.repository.ListUserMemberships(ctx, userID)
	if err != nil {
		return AuthUserRecord{}, nil, fmt.Errorf("load user company memberships: %w", err)
	}
	return record, memberships, nil
}

func (service *AuthenticationService) CreateUser(ctx context.Context, email, name, password, companyID string, role model.CompanyRole) (model.User, error) {
	email = normalizeEmail(email)
	id, err := newID("user")
	if err != nil {
		return model.User{}, err
	}
	user := model.User{
		ID:        id,
		Email:     email,
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := user.Validate(); err != nil {
		return model.User{}, err
	}
	if len(password) < minimumPassword || len(password) > maximumPassword {
		return model.User{}, fmt.Errorf("password must be between %d and %d bytes", minimumPassword, maximumPassword)
	}
	if strings.TrimSpace(companyID) == "" {
		return model.User{}, fmt.Errorf("company ID is required")
	}
	switch role {
	case model.CompanyRoleOwner, model.CompanyRoleAdmin, model.CompanyRoleManager, model.CompanyRoleOperator:
	default:
		return model.User{}, fmt.Errorf("unsupported company role %q", role)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}
	if err := service.repository.CreateUserWithMembership(ctx, user, string(hash), companyID, role); err != nil {
		return model.User{}, fmt.Errorf("create user and membership: %w", err)
	}
	return user, nil
}

func HashPassword(password string) (string, error) {
	if len(password) < minimumPassword || len(password) > maximumPassword {
		return "", fmt.Errorf("password must be between %d and %d bytes", minimumPassword, maximumPassword)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

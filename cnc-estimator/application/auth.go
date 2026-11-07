package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"smetchic/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user is inactive")
	ErrEmailAlreadyExists = errors.New("email is already registered")
)

const (
	bcryptCost           = 12
	minimumPasswordRunes = 6
	maximumPasswordBytes = 72
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
	RegisterCompanyWithOwner(context.Context, model.Company, model.CompanySettings, model.User, string) error
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
	if err := validatePassword(password); err != nil {
		return model.User{}, err
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
	if err := validatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (service *AuthenticationService) RegisterCompany(
	ctx context.Context,
	email, displayName, password, companyName string,
) (model.User, model.Company, error) {
	email = normalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	companyName = strings.TrimSpace(companyName)
	if displayName == "" {
		return model.User{}, model.Company{}, Invalid("Введите имя пользователя.", nil)
	}
	if companyName == "" {
		return model.User{}, model.Company{}, Invalid("Введите название компании.", nil)
	}
	if err := validatePassword(password); err != nil {
		return model.User{}, model.Company{}, Invalid(err.Error(), nil)
	}
	companyID, err := newID("company")
	if err != nil {
		return model.User{}, model.Company{}, err
	}
	userID, err := newID("user")
	if err != nil {
		return model.User{}, model.Company{}, err
	}
	now := time.Now().UTC()
	company := model.Company{ID: companyID, Name: companyName, CreatedAt: now, UpdatedAt: now}
	user := model.User{
		ID: userID, Email: email, Name: displayName,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := company.Validate(); err != nil {
		return model.User{}, model.Company{}, Invalid("Проверьте название компании.", err)
	}
	if err := user.Validate(); err != nil {
		return model.User{}, model.Company{}, Invalid("Проверьте email и имя пользователя.", err)
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return model.User{}, model.Company{}, Invalid(err.Error(), nil)
	}
	settings := model.CompanySettings{
		CompanyID: company.ID, Currency: "RUB", DefaultMarkupPercent: 50,
	}
	if err := service.repository.RegisterCompanyWithOwner(ctx, company, settings, user, passwordHash); err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			return model.User{}, model.Company{}, ErrEmailAlreadyExists
		}
		return model.User{}, model.Company{}, fmt.Errorf("register company and owner: %w", err)
	}
	return user, company, nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minimumPasswordRunes {
		return fmt.Errorf("Пароль должен содержать минимум %d символов.", minimumPasswordRunes)
	}
	if len(password) > maximumPasswordBytes {
		return fmt.Errorf("Пароль слишком длинный.")
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

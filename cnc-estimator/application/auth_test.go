package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smetchic/model"
)

type authTestRepository struct {
	record      AuthUserRecord
	memberships []AuthMembership
	createdHash string
}

func (repository *authTestRepository) GetAuthUserByEmail(_ context.Context, email string) (AuthUserRecord, error) {
	if strings.EqualFold(repository.record.User.Email, email) {
		return repository.record, nil
	}
	return AuthUserRecord{}, ErrNotFound
}

func (repository *authTestRepository) GetAuthUserByID(_ context.Context, userID string) (AuthUserRecord, error) {
	if repository.record.User.ID == userID {
		return repository.record, nil
	}
	return AuthUserRecord{}, ErrNotFound
}

func (repository *authTestRepository) ListUserMemberships(context.Context, string) ([]AuthMembership, error) {
	return repository.memberships, nil
}

func (repository *authTestRepository) CreateUserWithMembership(
	_ context.Context, user model.User, passwordHash, _ string, _ model.CompanyRole,
) error {
	if repository.record.User.Email == user.Email {
		return errors.New("duplicate email")
	}
	repository.createdHash = passwordHash
	repository.record = AuthUserRecord{User: user, PasswordHash: passwordHash, Active: true}
	return nil
}

func (repository *authTestRepository) RegisterCompanyWithOwner(
	_ context.Context, company model.Company, _ model.CompanySettings, user model.User, passwordHash string,
) error {
	if repository.record.User.Email == user.Email {
		return ErrEmailAlreadyExists
	}
	repository.record = AuthUserRecord{User: user, PasswordHash: passwordHash, Active: true}
	repository.memberships = []AuthMembership{{Company: company, Role: model.CompanyRoleOwner}}
	return nil
}

func TestPasswordHashVerification(t *testing.T) {
	hash, err := HashPassword("safe-password-for-test")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "safe-password-for-test" || strings.Contains(hash, "safe-password-for-test") {
		t.Fatal("password hash contains plaintext password")
	}
	if !VerifyPassword(hash, "safe-password-for-test") {
		t.Fatal("correct password did not verify")
	}
	if VerifyPassword(hash, "incorrect-password") {
		t.Fatal("incorrect password verified")
	}
}

func TestLoginUsesGenericCredentialError(t *testing.T) {
	hash, err := HashPassword("correct-password-test")
	if err != nil {
		t.Fatal(err)
	}
	repository := &authTestRepository{
		record: AuthUserRecord{
			User:         model.User{ID: "user-1", Email: "person@example.com"},
			PasswordHash: hash, Active: true,
		},
	}
	service := NewAuthenticationService(repository)
	if _, err := service.Login(context.Background(), " PERSON@example.com ", "correct-password-test"); err != nil {
		t.Fatalf("valid login failed: %v", err)
	}
	if _, err := service.Login(context.Background(), "person@example.com", "wrong-password-test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, err := service.Login(context.Background(), "missing@example.com", "wrong-password-test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email error = %v", err)
	}
	repository.record.Active = false
	if _, err := service.Login(context.Background(), "person@example.com", "correct-password-test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("inactive user error = %v", err)
	}
}

func TestCreateUserStoresOnlyPasswordHash(t *testing.T) {
	repository := &authTestRepository{}
	user, err := NewAuthenticationService(repository).CreateUser(
		context.Background(), " New@Example.com ", "New user", "long-password-for-test",
		"company-1", model.CompanyRoleOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "new@example.com" {
		t.Fatalf("normalized email = %q", user.Email)
	}
	if repository.createdHash == "" || repository.createdHash == "long-password-for-test" ||
		!VerifyPassword(repository.createdHash, "long-password-for-test") {
		t.Fatal("created record did not receive a valid one-way password hash")
	}
}

func TestPasswordLengthCountsCharacters(t *testing.T) {
	if _, err := HashPassword("abcdef"); err != nil {
		t.Fatalf("six ASCII characters should be accepted: %v", err)
	}
	if _, err := HashPassword("short"); err == nil || !strings.Contains(err.Error(), "6 символов") {
		t.Fatalf("short password error = %v", err)
	}
}

func TestRegisterCompanyCreatesOwnerAndRejectsDuplicateEmail(t *testing.T) {
	repository := &authTestRepository{}
	service := NewAuthenticationService(repository)
	user, company, err := service.RegisterCompany(
		context.Background(), " owner@example.test ", "Owner", "пароль", "My company",
	)
	if err != nil {
		t.Fatal(err)
	}
	if company.Name != "My company" || user.Email != "owner@example.test" ||
		len(repository.memberships) != 1 || repository.memberships[0].Role != model.CompanyRoleOwner {
		t.Fatalf("registration state: user=%#v company=%#v memberships=%#v", user, company, repository.memberships)
	}
	if repository.record.PasswordHash == "пароль" || !VerifyPassword(repository.record.PasswordHash, "пароль") {
		t.Fatal("registration stored plaintext instead of a password hash")
	}
	if _, _, err := service.RegisterCompany(
		context.Background(), user.Email, "Second", "another1", "Another company",
	); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("duplicate email error = %v", err)
	}
}

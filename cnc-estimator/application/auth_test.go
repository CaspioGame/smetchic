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

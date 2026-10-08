package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepository struct {
	user *domain.User
	err  error
}

func (m *mockUserRepository) GetByUsername(
	ctx context.Context,
	username string,
) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.user, nil
}

func (m *mockUserRepository) CreateUser(
	ctx context.Context,
	user *domain.User,
) (*domain.User, error) {
	return user, nil
}

func TestUserService_Login(t *testing.T) {
	password := "12345678"

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "alex",
		PasswordHash: string(passwordHash),
	}

	repository := &mockUserRepository{
		user: user,
	}

	service := NewUserService(repository)

	result, err := service.Login(
		context.Background(),
		"alex",
		password,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Username != "alex" {
		t.Fatalf(
			"expected username %q, got %q",
			"alex",
			result.Username,
		)
	}
}

func TestUserService_LoginInvalidPassword(t *testing.T) {
	password := "12345678"

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Username:     "alex",
		PasswordHash: string(passwordHash),
	}

	repository := &mockUserRepository{
		user: user,
	}

	service := NewUserService(repository)

	_, err = service.Login(
		context.Background(),
		"alex",
		"wrong-password",
	)

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestUserService_LoginUserNotFound(t *testing.T) {
	repository := &mockUserRepository{
		err: errors.New("user not found"),
	}

	service := NewUserService(repository)

	_, err := service.Login(
		context.Background(),
		"unknown",
		"12345678",
	)

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}
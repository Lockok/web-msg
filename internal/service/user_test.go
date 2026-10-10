package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepository struct {
	user        *domain.User
	err         error
	createCalls int
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
	m.createCalls++
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
		err: domain.ErrUserNotFound,
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

func TestUserService_LoginRepositoryError(t *testing.T) {
	dbErr := errors.New("database unavailable")

	repository := &mockUserRepository{
		err: dbErr,
	}

	service := NewUserService(repository)

	_, err := service.Login(
		context.Background(),
		"alex",
		"12345678",
	)

	if !errors.Is(err, dbErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	if errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("repository error must not become invalid credentials")
	}
}

func TestUserService_RegisterInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"empty username", "", "12345678"},
		{"short username", "ab", "12345678"},
		{"long username", strings.Repeat("a", 51), "12345678"},
		{"username with space", "alex smith", "12345678"},
		{"username with punctuation", "alex!", "12345678"},
		{"empty password", "alex", ""},
		{"short password", "alex", "1234567"},
		{"long password", "alex", strings.Repeat("a", 73)},
		{"Unicode password over byte limit", "alex", strings.Repeat("я", 37)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockUserRepository{}
			service := NewUserService(repository)

			user, err := service.Register(
				context.Background(),
				tt.username,
				tt.password,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if user != nil {
				t.Fatal("expected no user for invalid input")
			}

			if repository.createCalls != 0 {
				t.Fatal("repository must not be called for invalid input")
			}
		})
	}
}

func TestUserService_RegisterValidInput(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"minimum lengths", "abc", "12345678"},
		{"maximum lengths", strings.Repeat("a", 50), strings.Repeat("b", 72)},
		{"Unicode password", "alex_123", "пароль12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockUserRepository{}
			service := NewUserService(repository)

			user, err := service.Register(
				context.Background(),
				tt.username,
				tt.password,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if user == nil {
				t.Fatal("expected created user")
			}

			if user.ID == uuid.Nil {
				t.Error("expected non-empty user ID")
			}

			if user.Username != tt.username {
				t.Error("unexpected username")
			}

			if repository.createCalls != 1 {
				t.Errorf("expected one repository call, got %d", repository.createCalls)
			}

			if err := bcrypt.CompareHashAndPassword(
				[]byte(user.PasswordHash),
				[]byte(tt.password),
			); err != nil {
				t.Error("stored hash does not match password")
			}
		})
	}
}

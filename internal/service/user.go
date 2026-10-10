package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{
		users: users,
	}
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,50}$`)

func validateRegistration(username, password string) error {
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf(
			"%w: username must contain 3–50 Latin letters, digits or underscores",
			domain.ErrInvalidInput,
		)
	}

	if utf8.RuneCountInString(password) < 8 {
		return fmt.Errorf(
			"%w: password must contain at least 8 characters",
			domain.ErrInvalidInput,
		)
	}

	if len(password) > 72 {
		return fmt.Errorf(
			"%w: password must not exceed 72 bytes",
			domain.ErrInvalidInput,
		)
	}

	return nil
}

func (s *UserService) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.users.GetByUsername(ctx, username)
}

func (s *UserService) Register(ctx context.Context, username string, password string) (*domain.User, error) {
	if err := validateRegistration(username, password); err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: string(passwordHash),
	}

	createdUser, err := s.users.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (s *UserService) Login(
	ctx context.Context,
	username string,
	password string,
) (*domain.User, error) {
	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}

		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return user, nil
}

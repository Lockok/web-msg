package service

import (
	"context"

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

func (s *UserService) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.users.GetByUsername(ctx, username)
}

func (s *UserService) Register(ctx context.Context, username string, password string) (*domain.User, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID: uuid.New(),
		Username: username,
		PasswordHash: string(passwordHash),
	}

	createdUser, err := s.users.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}


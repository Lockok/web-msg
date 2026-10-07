package repository

import (
	"context"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
)

type UserRepository struct {
	queries *db.Queries
}

func NewUserRepository(queries *db.Queries) *UserRepository {
	return &UserRepository{
		queries: queries,
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	createdUser, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		ID: user.ID,
		Username: user.Username,
		PasswordHash: user.PasswordHash,
	})
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID: createdUser.ID,
		Username: createdUser.Username,
		PasswordHash: createdUser.PasswordHash,
		CreatedAt: createdUser.CreatedAt.Time,
	}, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	user, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:           user.ID,
		Username:     user.Username,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

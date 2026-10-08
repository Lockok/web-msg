package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrUsernameExists = errors.New("username already exists")

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrUsernameExists = errors.New("username already exists")
var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrUsernameExists = errors.New("username already exists")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUserNotFound = errors.New("user not found")
var ErrInvalidInput = errors.New("invalid input")

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

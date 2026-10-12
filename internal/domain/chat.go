package domain

import (
	"time"

	"github.com/google/uuid"
)

type Chat struct {
	ID        uuid.UUID
	Type      string
	Name      string
	CreatedAt time.Time
}

type ChatCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

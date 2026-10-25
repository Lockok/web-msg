package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const MaxMessageTextBytes = 4096

var ErrChatNotFound = errors.New("chat not found")
var ErrNotChatMember = errors.New("user is not a chat member")
var ErrMessageConflict = errors.New("message idempotency conflict")

type Message struct {
	ID              uuid.UUID
	ChatID          uuid.UUID
	SenderID        uuid.UUID
	Text            string
	Seq             int64
	ClientMessageID uuid.UUID
	CreatedAt       time.Time
}

type SendMessageInput struct {
	ChatID          uuid.UUID
	SenderID        uuid.UUID
	ClientMessageID uuid.UUID
	Text            string
}

type MessagePage struct {
	Items         []Message
	NextBeforeSeq *int64
}

type SendMessageResult struct {
	Message *Message
	Created bool
}

package realtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

const eventWriteTimeout = 5 * time.Second

type MessageCreatedEvent struct {
	Type    string         `json:"type"`
	Message MessagePayload `json:"message"`
}

type MessagePayload struct {
	ID              uuid.UUID `json:"id"`
	ChatID          uuid.UUID `json:"chat_id"`
	SenderID        uuid.UUID `json:"sender_id"`
	ClientMessageID uuid.UUID `json:"client_message_id"`
	Text            string    `json:"text"`
	Seq             int64     `json:"seq"`
	CreatedAt       time.Time `json:"created_at"`
}

func WriteMessageCreated(
	ctx context.Context,
	conn *websocket.Conn,
	message domain.Message,
) error {
	if conn == nil {
		return errors.New("event connection must not be nil")
	}

	event := MessageCreatedEvent{
		Type: "message.created",
		Message: MessagePayload{
			ID:              message.ID,
			ChatID:          message.ChatID,
			SenderID:        message.SenderID,
			ClientMessageID: message.ClientMessageID,
			Text:            message.Text,
			Seq:             message.Seq,
			CreatedAt:       message.CreatedAt,
		},
	}

	writeCtx, cancel := context.WithTimeout(ctx, eventWriteTimeout)
	defer cancel()

	if err := wsjson.Write(writeCtx, conn, event); err != nil {
		return fmt.Errorf("write message.created event: %w", err)
	}

	return nil
}

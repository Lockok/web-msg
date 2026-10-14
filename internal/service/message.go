package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type MessageRepository interface {
	Send(ctx context.Context, input domain.SendMessageInput) (*domain.Message, error)
}

type MessageService struct {
	messages MessageRepository
}

func NewMessageService(messages MessageRepository) *MessageService {
	return &MessageService{
		messages: messages,
	}
}

func (s *MessageService) Send(ctx context.Context, input domain.SendMessageInput) (*domain.Message, error) {
	if input.ChatID == uuid.Nil ||
		input.SenderID == uuid.Nil ||
		input.ClientMessageID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: message IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if len(input.Text) > domain.MaxMessageTextBytes {
		return nil, fmt.Errorf(
			"%w: message text must not exceed %d bytes",
			domain.ErrInvalidInput,
			domain.MaxMessageTextBytes,
		)
	}

	if !utf8.ValidString(input.Text) {
		return nil, fmt.Errorf(
			"%w: message text must be valid UTF-8",
			domain.ErrInvalidInput,
		)
	}

	if strings.IndexByte(input.Text, 0) >= 0 {
		return nil, fmt.Errorf(
			"%w: message text must not contain NUL",
			domain.ErrInvalidInput,
		)
	}

	if strings.TrimSpace(input.Text) == "" {
		return nil, fmt.Errorf(
			"%w: message text must not be blank",
			domain.ErrInvalidInput,
		)
	}

	message, err := s.messages.Send(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return message, nil
}

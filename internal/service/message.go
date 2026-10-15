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
	ListChatMessages(ctx context.Context, chatID uuid.UUID, userID uuid.UUID, pageSize int32, beforeSeq *int64) ([]domain.Message, error)
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

func (s *MessageService) ListChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	limit int,
	beforeSeq *int64,
) (*domain.MessagePage, error) {
	if chatID == uuid.Nil || userID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: chat and user IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf(
			"%w: limit must be between 1 and 100",
			domain.ErrInvalidInput,
		)
	}

	if beforeSeq != nil && *beforeSeq <= 0 {
		return nil, fmt.Errorf(
			"%w: before_seq must be positive",
			domain.ErrInvalidInput,
		)
	}

	messages, err := s.messages.ListChatMessages(
		ctx,
		chatID,
		userID,
		int32(limit+1),
		beforeSeq,
	)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}

	if messages == nil {
		messages = make([]domain.Message, 0)
	}

	page := &domain.MessagePage{
		Items: messages,
	}

	if len(messages) > limit {
		page.Items = messages[:limit]

		lastSeq := page.Items[len(page.Items)-1].Seq
		page.NextBeforeSeq = &lastSeq
	}

	return page, nil
}

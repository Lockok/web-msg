package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type MessageRepository interface {
	SendWithResult(ctx context.Context, input domain.SendMessageInput) (*domain.SendMessageResult, error)
	ListChatMessages(ctx context.Context, chatID uuid.UUID, userID uuid.UUID, pageSize int32, beforeSeq *int64) ([]domain.Message, error)
}

type MessageCreatedNotifer interface {
	NotifyMessageCreated(ctx context.Context, message domain.Message) error
}

type MessageService struct {
	messages MessageRepository
	notifer  MessageCreatedNotifer
	log      *slog.Logger
}

func NewMessageService(messages MessageRepository) *MessageService {
	return NewMessageServiceWithNotifer(
		messages,
		nil,
		slog.Default(),
	)
}

func NewMessageServiceWithNotifer(messages MessageRepository, notifer MessageCreatedNotifer, logger *slog.Logger) *MessageService {
	if logger == nil {
		logger = slog.Default()
	}

	return &MessageService{
		messages: messages,
		notifer:  notifer,
		log:      logger,
	}
}

func (s *MessageService) SendWithResult(ctx context.Context, input domain.SendMessageInput) (*domain.SendMessageResult, error) {
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

	result, err := s.messages.SendWithResult(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	if result.Created && s.notifer != nil {
		notifyCtx := context.WithoutCancel(ctx)

		err := s.notifer.NotifyMessageCreated(
			notifyCtx,
			*result.Message,
		)
		if err != nil {
			s.log.ErrorContext(
				ctx,
				"failed to notify about created message",
				"message_id", result.Message.ID,
				"chat_id", result.Message.ChatID,
				"error", err,
			)
		}
	}

	return result, nil
}

func (s *MessageService) Send(ctx context.Context, input domain.SendMessageInput) (*domain.Message, error) {
	result, err := s.SendWithResult(ctx, input)
	if err != nil {
		return nil, err
	}

	return result.Message, nil
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

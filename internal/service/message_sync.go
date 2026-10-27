package service

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (s *MessageService) SyncChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	afterSeq int64,
	limit int,
) (*domain.MessageSyncPage, error) {
	if chatID == uuid.Nil || userID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: chat and user IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if afterSeq < 0 {
		return nil, fmt.Errorf(
			"%w: after_seq must not be negative",
			domain.ErrInvalidInput,
		)
	}

	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf(
			"%w: limit must be between 1 and 100",
			domain.ErrInvalidInput,
		)
	}

	messages, err := s.messages.ListChatMessagesAfter(
		ctx,
		chatID,
		userID,
		afterSeq,
		int32(limit+1),
	)
	if err != nil {
		return nil, fmt.Errorf("sync chat messages: %w", err)
	}

	if messages == nil {
		messages = make([]domain.Message, 0)
	}

	page := &domain.MessageSyncPage{
		Items: messages,
	}

	if len(messages) > limit {
		page.Items = messages[:limit]

		nextAfterSeq := page.Items[len(page.Items)-1].Seq
		page.NextAfterSeq = &nextAfterSeq
	}

	return page, nil
}

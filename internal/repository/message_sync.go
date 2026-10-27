package repository

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
)

func (r *MessageRepository) ListChatMessagesAfter(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	afterSeq int64,
	pageSize int32,
) ([]domain.Message, error) {
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

	if pageSize < 1 || pageSize > 101 {
		return nil, fmt.Errorf(
			"%w: page size must be between 1 and 101",
			domain.ErrInvalidInput,
		)
	}

	exists, err := r.queries.ChatExists(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("check sync chat existence: %w", err)
	}

	if !exists {
		return nil, domain.ErrChatNotFound
	}

	isMember, err := r.queries.IsChatMember(
		ctx,
		db.IsChatMemberParams{
			ChatID: chatID,
			UserID: userID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("check sync reader membership: %w", err)
	}

	if !isMember {
		return nil, domain.ErrNotChatMember
	}

	rows, err := r.queries.ListChatMessagesAfter(
		ctx,
		db.ListChatMessagesAfterParams{
			ChatID:   chatID,
			UserID:   userID,
			AfterSeq: afterSeq,
			PageSize: pageSize,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list messages after sequence: %w", err)
	}

	messages := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, *messageToDomain(row))
	}

	return messages, nil
}

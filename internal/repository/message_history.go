package repository

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
)

func (r *MessageRepository) ListChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	pageSize int32,
	beforeSeq *int64,
) ([]domain.Message, error) {
	if chatID == uuid.Nil || userID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: chat and user IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if pageSize < 1 || pageSize > 101 {
		return nil, fmt.Errorf(
			"%w: page size must be between 1 and 101",
			domain.ErrInvalidInput,
		)
	}

	if beforeSeq != nil && *beforeSeq <= 0 {
		return nil, fmt.Errorf(
			"%w: before_seq must be positive",
			domain.ErrInvalidInput,
		)
	}

	exists, err := r.queries.ChatExists(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("check history chat existence: %w", err)
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
		return nil, fmt.Errorf("check history membership: %w", err)
	}

	if !isMember {
		return nil, domain.ErrNotChatMember
	}

	var (
		rows []db.Message
	)

	if beforeSeq == nil {
		rows, err = r.queries.ListChatMessages(
			ctx,
			db.ListChatMessagesParams{
				ChatID:   chatID,
				UserID:   userID,
				PageSize: pageSize,
			},
		)
	} else {
		rows, err = r.queries.ListChatMessagesBefore(
			ctx,
			db.ListChatMessagesBeforeParams{
				ChatID:    chatID,
				UserID:    userID,
				BeforeSeq: *beforeSeq,
				PageSize:  pageSize,
			},
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}

	messages := make([]domain.Message, 0, len(rows))

	for _, row := range rows {
		messages = append(messages, *messageToDomain(row))
	}

	return messages, nil
}

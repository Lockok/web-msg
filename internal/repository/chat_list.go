package repository

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *ChatRepository) ListUserChats(
	ctx context.Context,
	userID uuid.UUID,
	pageSize int32,
	cursor *domain.ChatCursor,
) ([]domain.Chat, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: user ID must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if pageSize < 1 || pageSize > 101 {
		return nil, fmt.Errorf(
			"%w: page size must be between 1 and 101",
			domain.ErrInvalidInput,
		)
	}

	if cursor != nil &&
		(cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero()) {
		return nil, fmt.Errorf(
			"%w: invalid chat cursor",
			domain.ErrInvalidInput,
		)
	}

	var (
		rows []db.Chat
		err  error
	)

	if cursor == nil {
		rows, err = r.queries.ListUserChats(
			ctx,
			db.ListUserChatsParams{
				UserID:   userID,
				PageSize: pageSize,
			},
		)
	} else {
		rows, err = r.queries.ListUserChatsBefore(
			ctx,
			db.ListUserChatsBeforeParams{
				UserID: userID,
				CursorCreatedAt: pgtype.Timestamptz{
					Time:  cursor.CreatedAt,
					Valid: true,
				},
				CursorID: cursor.ID,
				PageSize: pageSize,
			},
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list user chats: %w", err)
	}

	chats := make([]domain.Chat, 0, len(rows))

	for _, row := range rows {
		chats = append(chats, *chatToDomain(row))
	}

	return chats, nil
}

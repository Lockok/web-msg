package repository

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (r *ChatRepository) ListChatMemberIDs(
	ctx context.Context,
	chatID uuid.UUID,
) ([]uuid.UUID, error) {
	if chatID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: chat ID must not be empty",
			domain.ErrInvalidInput,
		)
	}

	userIDs, err := r.queries.ListChatMemberIDs(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list chat member IDs: %w", err)
	}

	if userIDs == nil {
		userIDs = make([]uuid.UUID, 0)
	}

	return userIDs, nil
}

package service

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (s *ChatService) ClearChatHistory(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (int64, error) {
	if chatID == uuid.Nil || userID == uuid.Nil {
		return 0, fmt.Errorf("%w: IDs must not be empty", domain.ErrInvalidInput)
	}

	clearedThrough, err := s.chats.ClearChatHistory(ctx, chatID, userID)
	if err != nil {
		return 0, fmt.Errorf("clear chat history: %w", err)
	}

	return clearedThrough, nil
}

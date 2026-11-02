package service

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (s *ChatService) SetGroupMember(
	ctx context.Context,
	chatID, actorID, targetID uuid.UUID,
	add bool,
) error {
	if chatID == uuid.Nil || actorID == uuid.Nil || targetID == uuid.Nil {
		return fmt.Errorf("%w: IDs must not be empty", domain.ErrInvalidInput)
	}
	if !add && actorID == targetID {
		return fmt.Errorf("%w: admin cannot remove themselves", domain.ErrInvalidInput)
	}

	if err := s.chats.SetGroupMember(ctx, chatID, actorID, targetID, add); err != nil {
		return fmt.Errorf("set group member: %w", err)
	}
	return nil
}

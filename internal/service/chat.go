package service

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type ChatRepository interface {
	GetOrCreatePrivateChat(ctx context.Context, userID uuid.UUID, peerID uuid.UUID) (*domain.Chat, error)
}

type ChatService struct {
	chats ChatRepository
}

func NewChatService(chats ChatRepository) *ChatService {
	return &ChatService{
		chats: chats,
	}
}

func (s *ChatService) GetOrCreatePrivateChat(ctx context.Context, userID uuid.UUID, peerID uuid.UUID) (*domain.Chat, error) {
	if userID == uuid.Nil || peerID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: user IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if userID == peerID {
		return nil, fmt.Errorf(
			"%w: cannot create a private with yourself",
			domain.ErrInvalidInput,
		)
	}

	chat, err := s.chats.GetOrCreatePrivateChat(ctx, userID, peerID)
	if err != nil {
		return nil, fmt.Errorf("get or create private chat: %w", err)
	}

	return chat, nil

}

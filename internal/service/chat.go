package service

import (
	"context"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type ChatRepository interface {
	GetOrCreatePrivateChat(ctx context.Context, userID uuid.UUID, peerID uuid.UUID) (*domain.Chat, error)
	ListUserChats(ctx context.Context, userID uuid.UUID, pageSize int32, cursor *domain.ChatCursor) ([]domain.Chat, error)
	CreateGroupChat(ctx context.Context, input domain.CreateGroupChatInput) (*domain.Chat, error)
	SetGroupMember(ctx context.Context, chatID, actorID, targetID uuid.UUID, add bool) error
	ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) (int64, error)
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

func (s *ChatService) ListUserChats(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	cursor *domain.ChatCursor,
) (*domain.ChatPage, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: user ID must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf(
			"%w: limit must be between 1 and 100",
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

	chats, err := s.chats.ListUserChats(
		ctx,
		userID,
		int32(limit+1),
		cursor,
	)
	if err != nil {
		return nil, fmt.Errorf("list user chats: %w", err)
	}

	if chats == nil {
		chats = make([]domain.Chat, 0)
	}

	page := &domain.ChatPage{
		Items: chats,
	}

	if len(chats) > limit {
		page.Items = chats[:limit]

		last := page.Items[len(page.Items)-1]

		page.NextCursor = &domain.ChatCursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		}
	}

	return page, nil
}

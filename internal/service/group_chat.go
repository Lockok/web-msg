package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (s *ChatService) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	if input.CreatorID == uuid.Nil || !utf8.ValidString(input.Name) {
		return nil, fmt.Errorf("%w: invalid group input", domain.ErrInvalidInput)
	}

	input.Name = strings.TrimSpace(input.Name)

	if input.Name == "" ||
		utf8.RuneCountInString(input.Name) > 100 ||
		strings.ContainsRune(input.Name, '\x00') {
		return nil, fmt.Errorf(
			"%w: group name must contain 1 to 100 characters",
			domain.ErrInvalidInput,
		)
	}

	chat, err := s.chats.CreateGroupChat(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create group chat: %w", err)
	}

	return chat, nil
}

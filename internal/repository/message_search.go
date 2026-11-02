package repository

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
)

func (r *MessageRepository) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	pageSize int32,
	beforeSeq *int64,
) ([]domain.Message, error) {
	searchText = strings.TrimSpace(searchText)

	if chatID == uuid.Nil || userID == uuid.Nil ||
		searchText == "" || len(searchText) > 256 ||
		!utf8.ValidString(searchText) ||
		strings.ContainsRune(searchText, '\x00') ||
		pageSize < 1 || pageSize > 101 ||
		(beforeSeq != nil && *beforeSeq <= 0) {
		return nil, fmt.Errorf("%w: invalid search parameters", domain.ErrInvalidInput)
	}

	exists, err := r.queries.ChatExists(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("check search chat existence: %w", err)
	}
	if !exists {
		return nil, domain.ErrChatNotFound
	}

	isMember, err := r.queries.IsChatMember(ctx, db.IsChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("check search membership: %w", err)
	}
	if !isMember {
		return nil, domain.ErrNotChatMember
	}

	var cursor int64
	if beforeSeq != nil {
		cursor = *beforeSeq
	}

	rows, err := r.queries.SearchChatMessages(ctx, db.SearchChatMessagesParams{
		ChatID:     chatID,
		UserID:     userID,
		SearchText: searchText,
		BeforeSeq:  cursor,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("search chat messages: %w", err)
	}

	messages := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, *messageToDomain(row))
	}
	return messages, nil
}

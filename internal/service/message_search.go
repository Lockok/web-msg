package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func (s *MessageService) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	limit int,
	beforeSeq *int64,
) (*domain.MessagePage, error) {
	searchText = strings.TrimSpace(searchText)

	if chatID == uuid.Nil || userID == uuid.Nil ||
		searchText == "" || len(searchText) > 256 ||
		!utf8.ValidString(searchText) ||
		strings.ContainsRune(searchText, '\x00') ||
		limit < 1 || limit > 100 ||
		(beforeSeq != nil && *beforeSeq <= 0) {
		return nil, fmt.Errorf("%w: invalid search parameters", domain.ErrInvalidInput)
	}

	messages, err := s.messages.SearchChatMessages(
		ctx, chatID, userID, searchText, int32(limit+1), beforeSeq,
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}

	if messages == nil {
		messages = make([]domain.Message, 0)
	}

	page := &domain.MessagePage{Items: messages}

	if len(messages) > limit {
		page.Items = messages[:limit]
		next := page.Items[len(page.Items)-1].Seq
		page.NextBeforeSeq = &next
	}

	return page, nil
}

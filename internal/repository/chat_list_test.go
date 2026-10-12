package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestChatRepository_ListUserChatsInvalidInput(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name     string
		userID   uuid.UUID
		pageSize int32
		cursor   *domain.ChatCursor
	}{
		{
			name:     "empty user ID",
			userID:   uuid.Nil,
			pageSize: 20,
		},
		{
			name:     "zero page size",
			userID:   userID,
			pageSize: 0,
		},
		{
			name:     "negative page size",
			userID:   userID,
			pageSize: -1,
		},
		{
			name:     "page size over repository limit",
			userID:   userID,
			pageSize: 102,
		},
		{
			name:     "empty cursor ID",
			userID:   userID,
			pageSize: 20,
			cursor: &domain.ChatCursor{
				CreatedAt: time.Now().UTC(),
			},
		},
		{
			name:     "empty cursor timestamp",
			userID:   userID,
			pageSize: 20,
			cursor: &domain.ChatCursor{
				ID: uuid.New(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Неверные данные должны отклоняться до обращения к БД.
			repository := &ChatRepository{}

			chats, err := repository.ListUserChats(
				context.Background(),
				tt.userID,
				tt.pageSize,
				tt.cursor,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if chats != nil {
				t.Fatal("expected no result for invalid input")
			}
		})
	}
}

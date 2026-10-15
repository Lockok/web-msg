package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageRepository_ListChatMessagesInvalidInput(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()

	zeroSeq := int64(0)
	negativeSeq := int64(-1)

	tests := []struct {
		name      string
		chatID    uuid.UUID
		userID    uuid.UUID
		pageSize  int32
		beforeSeq *int64
	}{
		{
			name:     "empty chat ID",
			userID:   userID,
			pageSize: 20,
		},
		{
			name:     "empty user ID",
			chatID:   chatID,
			pageSize: 20,
		},
		{
			name:     "zero page size",
			chatID:   chatID,
			userID:   userID,
			pageSize: 0,
		},
		{
			name:     "negative page size",
			chatID:   chatID,
			userID:   userID,
			pageSize: -1,
		},
		{
			name:     "page size over maximum",
			chatID:   chatID,
			userID:   userID,
			pageSize: 102,
		},
		{
			name:      "zero before_seq",
			chatID:    chatID,
			userID:    userID,
			pageSize:  20,
			beforeSeq: &zeroSeq,
		},
		{
			name:      "negative before_seq",
			chatID:    chatID,
			userID:    userID,
			pageSize:  20,
			beforeSeq: &negativeSeq,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Проверка должна завершиться до обращения к БД.
			repository := &MessageRepository{}

			messages, err := repository.ListChatMessages(
				context.Background(),
				tt.chatID,
				tt.userID,
				tt.pageSize,
				tt.beforeSeq,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if messages != nil {
				t.Fatal("expected no result for invalid input")
			}
		})
	}
}

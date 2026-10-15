package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageService_ListChatMessagesPagination(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()

	rows := make([]domain.Message, 101)
	for i := range rows {
		rows[i] = domain.Message{
			ID:       uuid.New(),
			ChatID:   chatID,
			SenderID: userID,
			Text:     "message",
			Seq:      int64(101 - i),
		}
	}

	beforeSeq := int64(200)

	tests := []struct {
		name      string
		rows      []domain.Message
		limit     int
		beforeSeq *int64
		wantItems int
		wantNext  bool
	}{
		{
			name:  "empty history",
			limit: 20,
		},
		{
			name:      "less than limit",
			rows:      rows[:1],
			limit:     2,
			wantItems: 1,
		},
		{
			name:      "exactly limit",
			rows:      rows[:2],
			limit:     2,
			wantItems: 2,
		},
		{
			name:      "extra row means continuation",
			rows:      rows[:3],
			limit:     2,
			wantItems: 2,
			wantNext:  true,
		},
		{
			name:      "minimum limit with cursor",
			rows:      rows[:2],
			limit:     1,
			beforeSeq: &beforeSeq,
			wantItems: 1,
			wantNext:  true,
		},
		{
			name:      "maximum limit with continuation",
			rows:      rows,
			limit:     100,
			wantItems: 100,
			wantNext:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{history: tt.rows}
			service := NewMessageService(repository)

			page, err := service.ListChatMessages(
				context.Background(),
				chatID,
				userID,
				tt.limit,
				tt.beforeSeq,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if page == nil || page.Items == nil {
				t.Fatal("expected a page with a non-nil items slice")
			}

			if len(page.Items) != tt.wantItems {
				t.Fatalf("expected %d items, got %d", tt.wantItems, len(page.Items))
			}

			for i, message := range page.Items {
				if message != tt.rows[i] {
					t.Fatalf("unexpected message at position %d", i)
				}
			}

			if repository.historyCalls != 1 ||
				repository.gotHistoryChatID != chatID ||
				repository.gotReaderID != userID ||
				repository.gotHistoryPageSize != int32(tt.limit+1) ||
				repository.gotBeforeSeq != tt.beforeSeq {
				t.Fatal("repository received unexpected arguments or call count")
			}

			if !tt.wantNext {
				if page.NextBeforeSeq != nil {
					t.Fatal("unexpected continuation cursor")
				}
				return
			}

			if page.NextBeforeSeq == nil {
				t.Fatal("expected continuation cursor")
			}

			last := page.Items[len(page.Items)-1]
			if *page.NextBeforeSeq != last.Seq {
				t.Fatal("cursor must point to the last returned message")
			}
		})
	}
}

func TestMessageService_ListChatMessagesInvalidInput(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	zero := int64(0)
	negative := int64(-1)

	tests := []struct {
		name      string
		chatID    uuid.UUID
		userID    uuid.UUID
		limit     int
		beforeSeq *int64
	}{
		{"empty chat ID", uuid.Nil, userID, 20, nil},
		{"empty user ID", chatID, uuid.Nil, 20, nil},
		{"zero limit", chatID, userID, 0, nil},
		{"negative limit", chatID, userID, -1, nil},
		{"limit over maximum", chatID, userID, 101, nil},
		{"zero before_seq", chatID, userID, 20, &zero},
		{"negative before_seq", chatID, userID, 20, &negative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{}
			service := NewMessageService(repository)

			page, err := service.ListChatMessages(
				context.Background(),
				tt.chatID,
				tt.userID,
				tt.limit,
				tt.beforeSeq,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if page != nil {
				t.Fatal("expected no page for invalid input")
			}

			if repository.historyCalls != 0 {
				t.Fatal("repository must not be called for invalid input")
			}
		})
	}
}

func TestMessageService_ListChatMessagesRepositoryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"missing chat", domain.ErrChatNotFound},
		{"not a member", domain.ErrNotChatMember},
		{"database failure", errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{historyErr: tt.err}
			service := NewMessageService(repository)

			page, err := service.ListChatMessages(
				context.Background(),
				uuid.New(),
				uuid.New(),
				20,
				nil,
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf("expected original error, got %v", err)
			}

			if page != nil {
				t.Fatal("expected no page after repository failure")
			}

			if repository.historyCalls != 1 {
				t.Fatalf("expected one call, got %d", repository.historyCalls)
			}
		})
	}
}

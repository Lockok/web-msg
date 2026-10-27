package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageService_SyncChatMessagesPagination(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()

	rows := make([]domain.Message, 101)
	for i := range rows {
		rows[i] = domain.Message{
			ID:       uuid.New(),
			ChatID:   chatID,
			SenderID: userID,
			Seq:      int64(i + 3),
			Text:     "hello",
		}
	}

	tests := []struct {
		name      string
		rows      []domain.Message
		limit     int
		wantCount int
		wantNext  int64 // 0 означает отсутствие курсора.
	}{
		{
			name:  "empty result",
			limit: 2,
		},
		{
			name:      "less than limit",
			rows:      rows[:1],
			limit:     2,
			wantCount: 1,
		},
		{
			name:      "exactly limit",
			rows:      rows[:2],
			limit:     2,
			wantCount: 2,
		},
		{
			name:      "extra row means continuation",
			rows:      rows[:3],
			limit:     2,
			wantCount: 2,
			wantNext:  4,
		},
		{
			name:      "minimum limit",
			rows:      rows[:2],
			limit:     1,
			wantCount: 1,
			wantNext:  3,
		},
		{
			name:      "maximum limit",
			rows:      rows,
			limit:     100,
			wantCount: 100,
			wantNext:  102,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{
				syncMessages: tt.rows,
			}
			service := NewMessageService(repository)

			page, err := service.SyncChatMessages(
				context.Background(),
				chatID,
				userID,
				2,
				tt.limit,
			)
			if err != nil {
				t.Fatalf("sync messages: %v", err)
			}

			if page == nil || page.Items == nil {
				t.Fatal("expected page with non-nil items")
			}

			if len(page.Items) != tt.wantCount {
				t.Fatalf(
					"expected %d items, got %d",
					tt.wantCount,
					len(page.Items),
				)
			}

			for i, message := range page.Items {
				if message != tt.rows[i] {
					t.Fatalf("service changed message at position %d", i)
				}
			}

			if tt.wantNext == 0 {
				if page.NextAfterSeq != nil {
					t.Fatal("expected no continuation cursor")
				}
			} else {
				if page.NextAfterSeq == nil ||
					*page.NextAfterSeq != tt.wantNext {
					t.Fatalf("expected continuation cursor %d", tt.wantNext)
				}
			}

			if repository.syncCalls != 1 ||
				repository.gotSyncChatID != chatID ||
				repository.gotSyncUserID != userID ||
				repository.gotSyncAfterSeq != 2 ||
				repository.gotSyncPageSize != int32(tt.limit+1) {
				t.Fatal("unexpected repository call arguments")
			}
		})
	}
}

func TestMessageService_SyncChatMessagesInvalidInput(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name     string
		chatID   uuid.UUID
		userID   uuid.UUID
		afterSeq int64
		limit    int
	}{
		{"empty chat ID", uuid.Nil, userID, 0, 20},
		{"empty user ID", chatID, uuid.Nil, 0, 20},
		{"negative sequence", chatID, userID, -1, 20},
		{"zero limit", chatID, userID, 0, 0},
		{"negative limit", chatID, userID, 0, -1},
		{"oversized limit", chatID, userID, 0, 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{}
			service := NewMessageService(repository)

			page, err := service.SyncChatMessages(
				context.Background(),
				tt.chatID,
				tt.userID,
				tt.afterSeq,
				tt.limit,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected invalid input error, got %v", err)
			}

			if page != nil {
				t.Fatal("invalid input must not return a page")
			}

			if repository.syncCalls != 0 {
				t.Fatal("invalid input must not reach repository")
			}
		})
	}
}

func TestMessageService_SyncChatMessagesRepositoryResult(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	for _, wantErr := range []error{
		nil,
		domain.ErrChatNotFound,
		domain.ErrNotChatMember,
		databaseErr,
	} {
		repository := &mockMessageRepository{
			syncErr: wantErr,
		}
		service := NewMessageService(repository)

		page, err := service.SyncChatMessages(
			context.Background(),
			uuid.New(),
			uuid.New(),
			0,
			20,
		)

		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}

		if repository.syncCalls != 1 ||
			repository.gotSyncAfterSeq != 0 {
			t.Fatal("after_seq=0 must reach repository")
		}

		if wantErr != nil && page != nil {
			t.Fatal("repository failure must not return a page")
		}

		if wantErr == nil && (page == nil || page.Items == nil) {
			t.Fatal("expected empty page for successful empty result")
		}
	}
}

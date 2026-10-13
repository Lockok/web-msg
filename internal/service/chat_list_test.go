package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestChatService_ListUserChatsPagination(t *testing.T) {
	userID := uuid.New()
	stamp := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	chats := []domain.Chat{
		{ID: uuid.New(), Type: "private", CreatedAt: stamp},
		{ID: uuid.New(), Type: "private", CreatedAt: stamp.Add(-time.Minute)},
		{ID: uuid.New(), Type: "private", CreatedAt: stamp.Add(-2 * time.Minute)},
	}

	cursor := &domain.ChatCursor{
		ID:        uuid.New(),
		CreatedAt: stamp.Add(time.Minute),
	}

	tests := []struct {
		name      string
		rows      []domain.Chat
		limit     int
		cursor    *domain.ChatCursor
		wantItems int
		wantNext  bool
	}{
		{
			name:  "empty result",
			limit: 2,
		},
		{
			name:      "less than limit",
			rows:      chats[:1],
			limit:     2,
			wantItems: 1,
		},
		{
			name:      "exactly limit",
			rows:      chats[:2],
			limit:     2,
			wantItems: 2,
		},
		{
			name:      "extra row means next page",
			rows:      chats,
			limit:     2,
			wantItems: 2,
			wantNext:  true,
		},
		{
			name:      "minimum limit with cursor",
			rows:      chats[:2],
			limit:     1,
			cursor:    cursor,
			wantItems: 1,
			wantNext:  true,
		},
		{
			name:      "maximum limit",
			rows:      chats,
			limit:     100,
			wantItems: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockChatRepository{listChats: tt.rows}
			service := NewChatService(repository)

			page, err := service.ListUserChats(
				context.Background(),
				userID,
				tt.limit,
				tt.cursor,
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

			for i, chat := range page.Items {
				if chat != tt.rows[i] {
					t.Fatalf("unexpected chat at position %d", i)
				}
			}

			if repository.listCalls != 1 ||
				repository.gotListUserID != userID ||
				repository.gotPageSize != int32(tt.limit+1) ||
				repository.gotCursor != tt.cursor {
				t.Fatal("repository received unexpected arguments or call count")
			}

			if !tt.wantNext {
				if page.NextCursor != nil {
					t.Fatal("unexpected next cursor")
				}
				return
			}

			if page.NextCursor == nil {
				t.Fatal("expected next cursor")
			}

			last := page.Items[len(page.Items)-1]

			if page.NextCursor.ID != last.ID ||
				!page.NextCursor.CreatedAt.Equal(last.CreatedAt) {
				t.Fatal("cursor must point to the last returned chat")
			}
		})
	}
}

func TestChatService_ListUserChatsInvalidInput(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name   string
		userID uuid.UUID
		limit  int
		cursor *domain.ChatCursor
	}{
		{"empty user ID", uuid.Nil, 20, nil},
		{"zero limit", userID, 0, nil},
		{"negative limit", userID, -1, nil},
		{"limit over maximum", userID, 101, nil},
		{
			"empty cursor ID",
			userID,
			20,
			&domain.ChatCursor{CreatedAt: time.Now().UTC()},
		},
		{
			"empty cursor timestamp",
			userID,
			20,
			&domain.ChatCursor{ID: uuid.New()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockChatRepository{}
			service := NewChatService(repository)

			page, err := service.ListUserChats(
				context.Background(),
				tt.userID,
				tt.limit,
				tt.cursor,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if page != nil {
				t.Fatal("expected no page for invalid input")
			}

			if repository.listCalls != 0 {
				t.Fatal("repository must not be called for invalid input")
			}
		})
	}
}

func TestChatService_ListUserChatsRepositoryError(t *testing.T) {
	dbErr := errors.New("database unavailable")

	repository := &mockChatRepository{listErr: dbErr}
	service := NewChatService(repository)

	page, err := service.ListUserChats(
		context.Background(),
		uuid.New(),
		20,
		nil,
	)

	if !errors.Is(err, dbErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	if page != nil {
		t.Fatal("expected no page after repository failure")
	}

	if repository.listCalls != 1 {
		t.Fatalf("expected one repository call, got %d", repository.listCalls)
	}
}

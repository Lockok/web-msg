package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type mockChatRepository struct {
	chat          *domain.Chat
	err           error
	calls         int
	gotUserID     uuid.UUID
	gotPeerID     uuid.UUID
	listChats     []domain.Chat
	listErr       error
	listCalls     int
	gotListUserID uuid.UUID
	gotPageSize   int32
	gotCursor     *domain.ChatCursor
}

func (m *mockChatRepository) GetOrCreatePrivateChat(
	ctx context.Context,
	userID uuid.UUID,
	peerID uuid.UUID,
) (*domain.Chat, error) {
	m.calls++
	m.gotUserID = userID
	m.gotPeerID = peerID

	return m.chat, m.err
}

func (m *mockChatRepository) ListUserChats(
	ctx context.Context,
	userID uuid.UUID,
	pageSize int32,
	cursor *domain.ChatCursor,
) ([]domain.Chat, error) {
	m.listCalls++
	m.gotListUserID = userID
	m.gotPageSize = pageSize
	m.gotCursor = cursor

	return m.listChats, m.listErr
}

func TestChatService_GetOrCreatePrivateChat(t *testing.T) {
	userID := uuid.New()
	peerID := uuid.New()

	want := &domain.Chat{
		ID:   uuid.New(),
		Type: "private",
	}

	repository := &mockChatRepository{chat: want}
	service := NewChatService(repository)

	chat, err := service.GetOrCreatePrivateChat(
		context.Background(),
		userID,
		peerID,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chat != want {
		t.Fatal("expected the chat returned by repository")
	}

	if repository.calls != 1 {
		t.Fatalf("expected one repository call, got %d", repository.calls)
	}

	if repository.gotUserID != userID || repository.gotPeerID != peerID {
		t.Fatal("repository received incorrect user IDs")
	}
}

func TestChatService_GetOrCreatePrivateChatInvalidIDs(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name   string
		userID uuid.UUID
		peerID uuid.UUID
	}{
		{"empty user ID", uuid.Nil, userID},
		{"empty peer ID", userID, uuid.Nil},
		{"same user", userID, userID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockChatRepository{}
			service := NewChatService(repository)

			chat, err := service.GetOrCreatePrivateChat(
				context.Background(),
				tt.userID,
				tt.peerID,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if chat != nil {
				t.Fatal("expected no chat for invalid IDs")
			}

			if repository.calls != 0 {
				t.Fatal("repository must not be called for invalid IDs")
			}
		})
	}
}

func TestChatService_GetOrCreatePrivateChatRepositoryError(t *testing.T) {
	dbErr := errors.New("database unavailable")

	tests := []struct {
		name string
		err  error
	}{
		{"missing user", domain.ErrUserNotFound},
		{"database failure", dbErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockChatRepository{err: tt.err}
			service := NewChatService(repository)

			chat, err := service.GetOrCreatePrivateChat(
				context.Background(),
				uuid.New(),
				uuid.New(),
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf("expected original error, got %v", err)
			}

			if chat != nil {
				t.Fatal("expected no chat after repository failure")
			}

			if repository.calls != 1 {
				t.Fatalf(
					"expected one repository call, got %d",
					repository.calls,
				)
			}
		})
	}
}

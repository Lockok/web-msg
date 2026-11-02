package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

// Реализация для существующих тестов других методов ChatService.
func (m *mockChatRepository) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	return nil, errors.New("unexpected CreateGroupChat call")
}

type groupChatRepositoryStub struct {
	*mockChatRepository
	calls int
	input domain.CreateGroupChatInput
	chat  *domain.Chat
	err   error
}

func (m *groupChatRepositoryStub) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	m.calls++
	m.input = input
	return m.chat, m.err
}

func (m *mockChatRepository) SetGroupMember(
	ctx context.Context,
	chatID, actorID, targetID uuid.UUID,
	add bool,
) error {
	return errors.New("unexpected SetGroupMember call")
}

func TestChatService_CreateGroupChat(t *testing.T) {
	creatorID := uuid.New()
	want := &domain.Chat{ID: uuid.New(), Type: "group", Name: "Команда"}
	repo := &groupChatRepositoryStub{chat: want}
	svc := NewChatService(repo)

	chat, err := svc.CreateGroupChat(context.Background(), domain.CreateGroupChatInput{
		CreatorID: creatorID,
		Name:      "  Команда  ",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if chat != want || repo.calls != 1 {
		t.Fatal("expected the repository result and one call")
	}
	if repo.input.CreatorID != creatorID || repo.input.Name != "Команда" {
		t.Fatalf("unexpected repository input: %+v", repo.input)
	}
}

func TestChatService_CreateGroupChatValidation(t *testing.T) {
	tests := []struct {
		name      string
		creatorID uuid.UUID
		groupName string
		valid     bool
	}{
		{"empty creator", uuid.Nil, "group", false},
		{"empty name", uuid.New(), "", false},
		{"whitespace", uuid.New(), "  \t ", false},
		{"invalid UTF-8", uuid.New(), string([]byte{0xff}), false},
		{"NUL", uuid.New(), "a\x00b", false},
		{"100 Unicode characters", uuid.New(), strings.Repeat("я", 100), true},
		{"101 Unicode characters", uuid.New(), strings.Repeat("я", 101), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &groupChatRepositoryStub{chat: &domain.Chat{}}
			svc := NewChatService(repo)

			_, err := svc.CreateGroupChat(context.Background(), domain.CreateGroupChatInput{
				CreatorID: tt.creatorID,
				Name:      tt.groupName,
			})

			if tt.valid {
				if err != nil || repo.calls != 1 {
					t.Fatalf("expected success and one call, err=%v calls=%d", err, repo.calls)
				}
				return
			}

			if !errors.Is(err, domain.ErrInvalidInput) || repo.calls != 0 {
				t.Fatalf("expected validation error and no call, err=%v calls=%d", err, repo.calls)
			}
		})
	}
}

func TestChatService_CreateGroupChatRepositoryError(t *testing.T) {
	repo := &groupChatRepositoryStub{err: domain.ErrUserNotFound}
	svc := NewChatService(repo)

	chat, err := svc.CreateGroupChat(context.Background(), domain.CreateGroupChatInput{
		CreatorID: uuid.New(),
		Name:      "group",
	})
	if chat != nil || !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected wrapped ErrUserNotFound, chat=%v err=%v", chat, err)
	}
}

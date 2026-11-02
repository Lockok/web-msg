//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageRepository_SearchChatMessagesIntegration(t *testing.T) {
	repo, pool, chatID, senderID := newMessageTestFixture(t)
	outsiderID := createChatTestUser(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	texts := []string{
		"Alpha",
		"nothing",
		"ALPHA two",
		"alpha%_ end",
		"ALPHABET",
	}

	for _, text := range texts {
		_, err := repo.Send(ctx, domain.SendMessageInput{
			ChatID:          chatID,
			SenderID:        senderID,
			ClientMessageID: uuid.New(),
			Text:            text,
		})
		if err != nil {
			t.Fatalf("send %q: %v", text, err)
		}
	}

	checkSeqs := func(got []domain.Message, want ...int64) {
		t.Helper()

		if got == nil {
			t.Fatal("expected a non-nil result slice")
		}
		if len(got) != len(want) {
			t.Fatalf("expected %d messages, got %d", len(want), len(got))
		}
		for i, seq := range want {
			if got[i].Seq != seq {
				t.Fatalf("position %d: expected seq %d, got %d", i, seq, got[i].Seq)
			}
		}
	}

	first, err := repo.SearchChatMessages(ctx, chatID, senderID, "alpha", 2, nil)
	if err != nil {
		t.Fatalf("search first page: %v", err)
	}
	checkSeqs(first, 5, 4)

	cursor := int64(4)
	second, err := repo.SearchChatMessages(ctx, chatID, senderID, "alpha", 2, &cursor)
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}
	checkSeqs(second, 3, 1)

	literal, err := repo.SearchChatMessages(ctx, chatID, senderID, "%_", 10, nil)
	if err != nil {
		t.Fatalf("search literal characters: %v", err)
	}
	checkSeqs(literal, 4)

	empty, err := repo.SearchChatMessages(ctx, chatID, senderID, "not present", 10, nil)
	if err != nil {
		t.Fatalf("search without matches: %v", err)
	}
	checkSeqs(empty)

	_, err = repo.SearchChatMessages(ctx, chatID, outsiderID, "alpha", 10, nil)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected outsider rejection, got %v", err)
	}

	_, err = repo.SearchChatMessages(ctx, uuid.New(), senderID, "alpha", 10, nil)
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("expected missing chat error, got %v", err)
	}

	_, err = pool.Exec(ctx, `
		DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2
	`, chatID, senderID)
	if err != nil {
		t.Fatalf("remove member: %v", err)
	}

	_, err = repo.SearchChatMessages(ctx, chatID, senderID, "alpha", 10, nil)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected removed member rejection, got %v", err)
	}
}

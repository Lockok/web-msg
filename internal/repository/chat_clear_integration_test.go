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

func TestChatRepository_ClearChatHistoryIntegration(t *testing.T) {
	messages, pool, chatID, userID := newMessageTestFixture(t)
	chats := NewChatRepository(pool)
	outsiderID := createChatTestUser(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var otherID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT user_id FROM chat_members
		WHERE chat_id = $1 AND user_id <> $2
	`, chatID, userID).Scan(&otherID)
	if err != nil {
		t.Fatalf("find other member: %v", err)
	}

	send := func(text string) {
		t.Helper()
		_, err := messages.Send(ctx, domain.SendMessageInput{
			ChatID:          chatID,
			SenderID:        userID,
			ClientMessageID: uuid.New(),
			Text:            text,
		})
		if err != nil {
			t.Fatalf("send message: %v", err)
		}
	}

	send("first")
	send("second")

	cutoff, err := chats.ClearChatHistory(ctx, chatID, userID)
	if err != nil || cutoff != 2 {
		t.Fatalf("expected cutoff 2, got %d, err=%v", cutoff, err)
	}

	own, err := messages.ListChatMessages(ctx, chatID, userID, 101, nil)
	if err != nil || len(own) != 0 {
		t.Fatalf("expected empty own history, messages=%v err=%v", own, err)
	}

	other, err := messages.ListChatMessages(ctx, chatID, otherID, 101, nil)
	if err != nil || len(other) != 2 {
		t.Fatalf("expected other member to retain history, messages=%v err=%v", other, err)
	}

	send("new")
	own, err = messages.ListChatMessages(ctx, chatID, userID, 101, nil)
	if err != nil || len(own) != 1 || own[0].Seq != 3 {
		t.Fatalf("expected new message, messages=%v err=%v", own, err)
	}

	// Повторная очистка включает сообщения, появившиеся после первой.
	cutoff, err = chats.ClearChatHistory(ctx, chatID, userID)
	if err != nil || cutoff != 3 {
		t.Fatalf("expected cutoff 3, got %d, err=%v", cutoff, err)
	}

	_, err = chats.ClearChatHistory(ctx, chatID, outsiderID)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected outsider rejection, got %v", err)
	}

	_, err = chats.ClearChatHistory(ctx, uuid.New(), userID)
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("expected missing chat error, got %v", err)
	}

	var storedCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM messages WHERE chat_id = $1
	`, chatID).Scan(&storedCount)
	if err != nil || storedCount != 3 {
		t.Fatalf("messages must remain in DB: count=%d err=%v", storedCount, err)
	}
}

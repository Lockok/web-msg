//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageRepository_ListChatMessagesPaginationIntegration(t *testing.T) {
	repository, pool, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	expectedIDs := make(map[int64]uuid.UUID)

	for i := 1; i <= 5; i++ {
		message, err := repository.Send(ctx, domain.SendMessageInput{
			ChatID:          chatID,
			SenderID:        senderID,
			ClientMessageID: uuid.New(),
			Text:            fmt.Sprintf("message %d", i),
		})
		if err != nil {
			t.Fatalf("send fixture message: %v", err)
		}

		if message == nil || message.Seq != int64(i) {
			t.Fatalf("expected message with sequence %d", i)
		}

		expectedIDs[message.Seq] = message.ID
	}

	// Одинаковое время: порядок должен определяться seq.
	stamp := time.Date(2026, 10, 11, 12, 0, 0, 123456000, time.UTC)

	_, err := pool.Exec(ctx, `
		UPDATE messages
		SET created_at = $1
		WHERE chat_id = $2`,
		stamp,
		chatID,
	)
	if err != nil {
		t.Fatalf("set equal timestamps: %v", err)
	}

	// Другой чат тоже доступен отправителю,
	// но его сообщения не должны попадать в историю первого.
	otherPeerID := createChatTestUser(t, pool)

	otherChat, err := NewChatRepository(pool).GetOrCreatePrivateChat(
		ctx,
		senderID,
		otherPeerID,
	)
	if err != nil {
		t.Fatalf("create other chat: %v", err)
	}

	_, err = repository.Send(ctx, domain.SendMessageInput{
		ChatID:          otherChat.ID,
		SenderID:        senderID,
		ClientMessageID: uuid.New(),
		Text:            "message from another chat",
	})
	if err != nil {
		t.Fatalf("send message to other chat: %v", err)
	}

	assertPage := func(messages []domain.Message, wantSeqs []int64) {
		t.Helper()

		if messages == nil {
			t.Fatal("expected a non-nil result slice")
		}

		if len(messages) != len(wantSeqs) {
			t.Fatalf("expected %d messages, got %d", len(wantSeqs), len(messages))
		}

		for i, wantSeq := range wantSeqs {
			message := messages[i]

			if message.Seq != wantSeq ||
				message.ID != expectedIDs[wantSeq] ||
				message.ChatID != chatID ||
				message.SenderID != senderID ||
				message.Text != fmt.Sprintf("message %d", wantSeq) ||
				!message.CreatedAt.Equal(stamp) {
				t.Fatalf("unexpected message at position %d: %+v", i, message)
			}
		}
	}

	first, err := repository.ListChatMessages(ctx, chatID, senderID, 2, nil)
	if err != nil {
		t.Fatalf("get first page: %v", err)
	}
	assertPage(first, []int64{5, 4})

	beforeSeq := first[len(first)-1].Seq

	second, err := repository.ListChatMessages(ctx, chatID, senderID, 2, &beforeSeq)
	if err != nil {
		t.Fatalf("get second page: %v", err)
	}
	assertPage(second, []int64{3, 2})

	beforeSeq = second[len(second)-1].Seq

	third, err := repository.ListChatMessages(ctx, chatID, senderID, 2, &beforeSeq)
	if err != nil {
		t.Fatalf("get third page: %v", err)
	}
	assertPage(third, []int64{1})

	beforeSeq = third[len(third)-1].Seq

	last, err := repository.ListChatMessages(ctx, chatID, senderID, 2, &beforeSeq)
	if err != nil {
		t.Fatalf("get page after oldest message: %v", err)
	}
	assertPage(last, nil)

	all, err := repository.ListChatMessages(ctx, chatID, senderID, 101, nil)
	if err != nil {
		t.Fatalf("get all fixture messages: %v", err)
	}
	assertPage(all, []int64{5, 4, 3, 2, 1})
}

func TestMessageRepository_ListChatMessagesEmptyIntegration(t *testing.T) {
	repository, _, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := repository.ListChatMessages(ctx, chatID, senderID, 20, nil)
	if err != nil {
		t.Fatalf("get empty history: %v", err)
	}

	if messages == nil || len(messages) != 0 {
		t.Fatal("expected an empty non-nil slice")
	}
}

func TestMessageRepository_ListChatMessagesAccessIntegration(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		wantError error
	}{
		{"missing chat", "missing", domain.ErrChatNotFound},
		{"outsider", "outsider", domain.ErrNotChatMember},
		{"removed member", "removed", domain.ErrNotChatMember},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, pool, chatID, senderID := newMessageTestFixture(t)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := repository.Send(ctx, domain.SendMessageInput{
				ChatID:          chatID,
				SenderID:        senderID,
				ClientMessageID: uuid.New(),
				Text:            "private message",
			})
			if err != nil {
				t.Fatalf("send fixture message: %v", err)
			}

			requestedChatID := chatID
			readerID := senderID

			switch tt.mode {
			case "missing":
				requestedChatID = uuid.New()

			case "outsider":
				readerID = createChatTestUser(t, pool)

			case "removed":
				_, err := pool.Exec(ctx, `
					DELETE FROM chat_members
					WHERE chat_id = $1 AND user_id = $2`,
					chatID,
					senderID,
				)
				if err != nil {
					t.Fatalf("remove member: %v", err)
				}
			}

			beforeSeq := int64(2)

			// Проверяем и первую страницу, и запрос с курсором.
			for _, cursor := range []*int64{nil, &beforeSeq} {
				messages, err := repository.ListChatMessages(
					ctx,
					requestedChatID,
					readerID,
					20,
					cursor,
				)

				if !errors.Is(err, tt.wantError) {
					t.Fatalf("expected %v, got %v", tt.wantError, err)
				}

				if messages != nil {
					t.Fatal("must not return history after access failure")
				}
			}
		})
	}
}

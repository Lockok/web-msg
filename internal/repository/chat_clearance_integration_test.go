//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestChatHistoryClearanceVisibilityIntegration(t *testing.T) {
	repo, pool, chatID, firstUserID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var secondUserID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT user_id
		FROM chat_members
		WHERE chat_id = $1 AND user_id <> $2
	`, chatID, firstUserID).Scan(&secondUserID)
	if err != nil {
		t.Fatalf("find second member: %v", err)
	}

	for i := 1; i <= 3; i++ {
		_, err := repo.Send(ctx, domain.SendMessageInput{
			ChatID:          chatID,
			SenderID:        firstUserID,
			ClientMessageID: uuid.New(),
			Text:            fmt.Sprintf("clear-test %d", i),
		})
		if err != nil {
			t.Fatalf("send message %d: %v", i, err)
		}
	}

	// Пока устанавливаем границу напрямую: API очистки ещё не создан.
	_, err = pool.Exec(ctx, `
		INSERT INTO chat_history_clearances
		    (chat_id, user_id, cleared_through_seq)
		VALUES ($1, $2, 2)
	`, chatID, firstUserID)
	if err != nil {
		t.Fatalf("set clearance: %v", err)
	}

	check := func(label string, got []domain.Message, want ...int64) {
		t.Helper()

		if len(got) != len(want) {
			t.Fatalf("%s: expected %d messages, got %d", label, len(want), len(got))
		}
		for i, seq := range want {
			if got[i].Seq != seq {
				t.Fatalf("%s: expected seq %d, got %d", label, seq, got[i].Seq)
			}
		}
	}

	// История и поиск идут от новых к старым.
	history, err := repo.ListChatMessages(ctx, chatID, firstUserID, 101, nil)
	if err != nil {
		t.Fatalf("first user history: %v", err)
	}
	check("cleared history", history, 3)

	search, err := repo.SearchChatMessages(
		ctx, chatID, firstUserID, "clear-test", 101, nil,
	)
	if err != nil {
		t.Fatalf("first user search: %v", err)
	}
	check("cleared search", search, 3)

	// Восстановление идёт от старых к новым.
	syncMessages, err := repo.ListChatMessagesAfter(
		ctx, chatID, firstUserID, 0, 101,
	)
	if err != nil {
		t.Fatalf("first user sync: %v", err)
	}
	check("cleared sync", syncMessages, 3)

	// У второго участника история не изменилась.
	history, err = repo.ListChatMessages(ctx, chatID, secondUserID, 101, nil)
	if err != nil {
		t.Fatalf("second user history: %v", err)
	}
	check("other user history", history, 3, 2, 1)

	search, err = repo.SearchChatMessages(
		ctx, chatID, secondUserID, "clear-test", 101, nil,
	)
	if err != nil {
		t.Fatalf("second user search: %v", err)
	}
	check("other user search", search, 3, 2, 1)

	syncMessages, err = repo.ListChatMessagesAfter(
		ctx, chatID, secondUserID, 0, 101,
	)
	if err != nil {
		t.Fatalf("second user sync: %v", err)
	}
	check("other user sync", syncMessages, 1, 2, 3)
}

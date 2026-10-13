//go:build integration

package repository

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestChatRepository_ListUserChatsPaginationIntegration(t *testing.T) {
	pool := newChatTestPool(t)

	userID := createChatTestUser(t, pool)
	emptyUserID := createChatTestUser(t, pool)

	peers := []uuid.UUID{
		createChatTestUser(t, pool),
		createChatTestUser(t, pool),
		createChatTestUser(t, pool),
	}

	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ids := make([]uuid.UUID, 0, len(peers))

	for _, peerID := range peers {
		chat, err := repository.GetOrCreatePrivateChat(ctx, userID, peerID)
		if err != nil {
			t.Fatalf("create accessible chat: %v", err)
		}

		ids = append(ids, chat.ID)
	}

	// Этот чат не принадлежит userID.
	foreignChat, err := repository.GetOrCreatePrivateChat(ctx, peers[0], peers[1])
	if err != nil {
		t.Fatalf("create foreign chat: %v", err)
	}

	// Микросекундная точность соответствует PostgreSQL.
	stamp := time.Date(2026, 10, 11, 12, 0, 0, 123456000, time.UTC)

	_, err = pool.Exec(ctx, `
		UPDATE chats
		SET created_at = $1
		WHERE id IN ($2, $3, $4)`,
		stamp,
		ids[0],
		ids[1],
		ids[2],
	)
	if err != nil {
		t.Fatalf("set equal chat timestamps: %v", err)
	}

	// Чужой чат делаем новее всех: без фильтра членства он попал бы первым.
	_, err = pool.Exec(ctx, `
		UPDATE chats
		SET created_at = $1
		WHERE id = $2`,
		stamp.Add(time.Hour),
		foreignChat.ID,
	)
	if err != nil {
		t.Fatalf("set foreign chat timestamp: %v", err)
	}

	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i][:], ids[j][:]) > 0
	})

	assertIDs := func(chats []domain.Chat, want []uuid.UUID) {
		t.Helper()

		if len(chats) != len(want) {
			t.Fatalf("expected %d chats, got %d", len(want), len(chats))
		}

		for i, chat := range chats {
			if chat.ID != want[i] {
				t.Fatalf(
					"position %d: expected chat %s, got %s",
					i,
					want[i],
					chat.ID,
				)
			}

			if chat.Type != "private" {
				t.Fatalf("unexpected chat type: %s", chat.Type)
			}
		}
	}

	// Первая страница: при равном времени ID идут по убыванию.
	first, err := repository.ListUserChats(ctx, userID, 2, nil)
	if err != nil {
		t.Fatalf("get first page: %v", err)
	}

	assertIDs(first, ids[:2])

	for _, chat := range first {
		if !chat.CreatedAt.Equal(stamp) {
			t.Fatalf("unexpected timestamp: %v", chat.CreatedAt)
		}
	}

	firstCursor := &domain.ChatCursor{
		CreatedAt: first[len(first)-1].CreatedAt,
		ID:        first[len(first)-1].ID,
	}

	// Вторая страница: оставшийся чат, без повторения предыдущих.
	second, err := repository.ListUserChats(ctx, userID, 2, firstCursor)
	if err != nil {
		t.Fatalf("get second page: %v", err)
	}

	assertIDs(second, ids[2:])

	secondCursor := &domain.ChatCursor{
		CreatedAt: second[0].CreatedAt,
		ID:        second[0].ID,
	}

	// После последнего чата результат пустой.
	last, err := repository.ListUserChats(ctx, userID, 2, secondCursor)
	if err != nil {
		t.Fatalf("get page after last chat: %v", err)
	}

	assertIDs(last, nil)

	if last == nil {
		t.Fatal("expected an empty slice, not nil")
	}

	// Пользователь без членства не видит ни одного чата.
	empty, err := repository.ListUserChats(ctx, emptyUserID, 20, nil)
	if err != nil {
		t.Fatalf("get chats for user without chats: %v", err)
	}

	assertIDs(empty, nil)

	if empty == nil {
		t.Fatal("expected an empty slice for user without chats")
	}

	// Чат с самым маленьким ID делаем новее остальных.
	// Теперь он должен стать первым: время важнее ID.
	newestID := ids[2]

	_, err = pool.Exec(ctx, `
		UPDATE chats
		SET created_at = $1
		WHERE id = $2`,
		stamp.Add(time.Minute),
		newestID,
	)
	if err != nil {
		t.Fatalf("change chat timestamp: %v", err)
	}

	all, err := repository.ListUserChats(ctx, userID, 101, nil)
	if err != nil {
		t.Fatalf("get chats with different timestamps: %v", err)
	}

	assertIDs(all, []uuid.UUID{newestID, ids[0], ids[1]})

	if !all[0].CreatedAt.Equal(stamp.Add(time.Minute)) {
		t.Fatal("newest chat has an unexpected timestamp")
	}
}

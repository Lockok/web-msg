//go:build integration

package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newChatTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database configuration: %v", err)
	}

	if cfg.ConnConfig.Database != "messenger_test" {
		t.Fatal("integration tests must use the messenger_test database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	return pool
}

func createChatTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	username := "test_" + id.String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(
		ctx,
		`INSERT INTO users (id, username, password_hash)
		 VALUES ($1, $2, $3)`,
		id,
		username,
		"test-only",
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Сначала удаляем связанные чаты: пары запрещают удаление пользователя.
		_, err := pool.Exec(ctx, `
			DELETE FROM chats
			WHERE id IN (
				SELECT chat_id FROM chat_members WHERE user_id = $1
				UNION
				SELECT chat_id FROM private_chat_pairs
				WHERE user_low_id = $1 OR user_high_id = $1
			)`,
			id,
		)
		if err != nil {
			t.Errorf("clean up test chats: %v", err)
			return
		}

		_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
		if err != nil {
			t.Errorf("clean up test user: %v", err)
		}
	})

	return id
}

type chatTableCounts struct {
	chats   int64
	pairs   int64
	members int64
}

func countChatTables(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
) chatTableCounts {
	t.Helper()

	var counts chatTableCounts

	err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM chats),
			(SELECT COUNT(*) FROM private_chat_pairs),
			(SELECT COUNT(*) FROM chat_members)
	`).Scan(&counts.chats, &counts.pairs, &counts.members)

	if err != nil {
		t.Fatalf("count chat tables: %v", err)
	}

	return counts
}

func TestChatRepository_GetOrCreatePrivateChatIntegration(t *testing.T) {
	pool := newChatTestPool(t)

	userID := createChatTestUser(t, pool)
	peerID := createChatTestUser(t, pool)

	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	before := countChatTables(t, ctx, pool)

	chat, err := repository.GetOrCreatePrivateChat(ctx, userID, peerID)
	if err != nil {
		t.Fatalf("create private chat: %v", err)
	}

	if chat.ID == uuid.Nil || chat.Type != "private" {
		t.Fatalf("unexpected chat: %+v", chat)
	}

	repeated, err := repository.GetOrCreatePrivateChat(ctx, peerID, userID)
	if err != nil {
		t.Fatalf("get private chat with reversed users: %v", err)
	}

	if repeated.ID != chat.ID {
		t.Fatalf("expected chat %s, got %s", chat.ID, repeated.ID)
	}

	after := countChatTables(t, ctx, pool)

	want := chatTableCounts{
		chats:   before.chats + 1,
		pairs:   before.pairs + 1,
		members: before.members + 2,
	}

	if after != want {
		t.Fatalf("expected table counts %+v, got %+v", want, after)
	}

	var lowID, highID uuid.UUID

	err = pool.QueryRow(ctx, `
		SELECT user_low_id, user_high_id
		FROM private_chat_pairs
		WHERE chat_id = $1`,
		chat.ID,
	).Scan(&lowID, &highID)
	if err != nil {
		t.Fatalf("get stored pair: %v", err)
	}

	wantLow, wantHigh := orderUserIDs(userID, peerID)
	if lowID != wantLow || highID != wantHigh {
		t.Fatal("stored pair does not match requested users")
	}

	var matchingMembers int

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM chat_members
		WHERE chat_id = $1
		  AND user_id IN ($2, $3)
		  AND role = 'member'`,
		chat.ID,
		userID,
		peerID,
	).Scan(&matchingMembers)
	if err != nil {
		t.Fatalf("check chat members: %v", err)
	}

	if matchingMembers != 2 {
		t.Fatalf("expected both users with member role, got %d", matchingMembers)
	}
}

func TestChatRepository_GetOrCreatePrivateChatRollbackIntegration(t *testing.T) {
	pool := newChatTestPool(t)

	userID := createChatTestUser(t, pool)
	missingPeerID := uuid.New()

	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	before := countChatTables(t, ctx, pool)

	chat, err := repository.GetOrCreatePrivateChat(
		ctx,
		userID,
		missingPeerID,
	)

	if err == nil {
		t.Fatal("expected error for a nonexistent peer")
	}

	if chat != nil {
		t.Fatal("must not return a chat after failed creation")
	}

	after := countChatTables(t, ctx, pool)

	if after != before {
		t.Fatalf(
			"failed transaction changed table counts: before=%+v, after=%+v",
			before,
			after,
		)
	}
}

func TestChatRepository_GetOrCreatePrivateChatConcurrentIntegration(t *testing.T) {
	pool := newChatTestPool(t)

	userID := createChatTestUser(t, pool)
	peerID := createChatTestUser(t, pool)

	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	before := countChatTables(t, ctx, pool)

	const workers = 12

	type result struct {
		chatID uuid.UUID
		err    error
	}

	start := make(chan struct{})
	results := make(chan result, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		a, b := userID, peerID
		if i%2 != 0 {
			a, b = b, a
		}

		wg.Add(1)

		go func(a, b uuid.UUID) {
			defer wg.Done()

			<-start

			chat, err := repository.GetOrCreatePrivateChat(ctx, a, b)

			r := result{err: err}
			if chat != nil {
				r.chatID = chat.ID
			}

			results <- r
		}(a, b)
	}

	close(start)

	wg.Wait()
	close(results)

	var commonChatID uuid.UUID
	successful := 0

	for r := range results {
		if r.err != nil {
			t.Errorf("concurrent request failed: %v", r.err)
			continue
		}

		if r.chatID == uuid.Nil {
			t.Error("concurrent request returned no chat ID")
			continue
		}

		successful++

		if commonChatID == uuid.Nil {
			commonChatID = r.chatID
		} else if r.chatID != commonChatID {
			t.Errorf(
				"expected common chat ID %s, got %s",
				commonChatID,
				r.chatID,
			)
		}
	}

	if successful != workers {
		t.Errorf("expected %d successful requests, got %d", workers, successful)
	}

	after := countChatTables(t, ctx, pool)

	want := chatTableCounts{
		chats:   before.chats + 1,
		pairs:   before.pairs + 1,
		members: before.members + 2,
	}

	if after != want {
		t.Fatalf("expected table counts %+v, got %+v", want, after)
	}
}

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

func TestChatRepository_CreateGroupChatIntegration(t *testing.T) {
	pool := newChatTestPool(t)
	creatorID := createChatTestUser(t, pool)
	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := "group-" + uuid.NewString()

	chat, err := repository.CreateGroupChat(ctx, domain.CreateGroupChatInput{
		CreatorID: creatorID,
		Name:      name,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if chat == nil {
		t.Fatal("expected a chat")
	}
	if chat.ID == uuid.Nil || chat.Type != "group" || chat.Name != name {
		t.Fatalf("unexpected chat: %+v", chat)
	}

	var storedType, storedName string
	err = pool.QueryRow(ctx, `
		SELECT type, name FROM chats WHERE id = $1
	`, chat.ID).Scan(&storedType, &storedName)
	if err != nil {
		t.Fatalf("load group: %v", err)
	}
	if storedType != "group" || storedName != name {
		t.Fatalf("unexpected stored group: type=%q name=%q", storedType, storedName)
	}

	var memberCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM chat_members WHERE chat_id = $1
	`, chat.ID).Scan(&memberCount)
	if err != nil {
		t.Fatalf("count members: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("expected one member, got %d", memberCount)
	}

	var role string
	err = pool.QueryRow(ctx, `
		SELECT role FROM chat_members WHERE chat_id = $1 AND user_id = $2
	`, chat.ID, creatorID).Scan(&role)
	if err != nil {
		t.Fatalf("load creator membership: %v", err)
	}
	if role != "admin" {
		t.Fatalf("expected admin role, got %q", role)
	}
}

func TestChatRepository_CreateGroupChatRollbackIntegration(t *testing.T) {
	pool := newChatTestPool(t)
	repository := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Уникальное имя позволяет проверить именно запись этого теста,
	// не сравнивая общее количество чатов в базе.
	name := "rollback-" + uuid.NewString()

	// Очистка на случай ошибки реализации, оставившей группу в базе.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cleanupCancel()

		if _, err := pool.Exec(
			cleanupCtx,
			`DELETE FROM chats WHERE name = $1`,
			name,
		); err != nil {
			t.Errorf("clean up group: %v", err)
		}
	})

	chat, err := repository.CreateGroupChat(ctx, domain.CreateGroupChatInput{
		CreatorID: uuid.New(), // Такого пользователя в базе нет.
		Name:      name,
	})
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if chat != nil {
		t.Fatalf("expected no chat on failure, got %+v", chat)
	}

	var count int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM chats WHERE name = $1
	`, name).Scan(&count)
	if err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to remove group, found %d", count)
	}
}

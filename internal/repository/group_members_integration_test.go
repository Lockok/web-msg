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

func TestChatRepository_SetGroupMemberIntegration(t *testing.T) {
	pool := newChatTestPool(t)
	adminID := createChatTestUser(t, pool)
	memberID := createChatTestUser(t, pool)
	outsiderID := createChatTestUser(t, pool)
	repo := NewChatRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	group, err := repo.CreateGroupChat(ctx, domain.CreateGroupChatInput{
		CreatorID: adminID,
		Name:      "members-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Посторонний не может добавлять участников.
	err = repo.SetGroupMember(ctx, group.ID, outsiderID, memberID, true)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected forbidden for outsider, got %v", err)
	}

	// Администратор добавляет участника; повторный вызов безопасен.
	for i := 0; i < 2; i++ {
		if err := repo.SetGroupMember(ctx, group.ID, adminID, memberID, true); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	var role string
	err = pool.QueryRow(ctx, `
		SELECT role FROM chat_members WHERE chat_id = $1 AND user_id = $2
	`, group.ID, memberID).Scan(&role)
	if err != nil || role != "member" {
		t.Fatalf("expected member role, role=%q err=%v", role, err)
	}

	// Обычный участник не может управлять составом.
	err = repo.SetGroupMember(ctx, group.ID, memberID, outsiderID, true)
	if !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected forbidden for member, got %v", err)
	}

	err = repo.SetGroupMember(ctx, group.ID, adminID, adminID, false)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected self-removal error, got %v", err)
	}

	err = repo.SetGroupMember(ctx, group.ID, adminID, uuid.New(), true)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected missing user error, got %v", err)
	}

	// Удаление тоже идемпотентно.
	for i := 0; i < 2; i++ {
		if err := repo.SetGroupMember(ctx, group.ID, adminID, memberID, false); err != nil {
			t.Fatalf("remove member: %v", err)
		}
	}

	var count int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM chat_members WHERE chat_id = $1 AND user_id = $2
	`, group.ID, memberID).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("expected removed member, count=%d err=%v", count, err)
	}

	err = repo.SetGroupMember(ctx, uuid.New(), adminID, memberID, true)
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("expected missing chat error, got %v", err)
	}

	privateChat, err := repo.GetOrCreatePrivateChat(ctx, adminID, outsiderID)
	if err != nil {
		t.Fatalf("create private chat: %v", err)
	}
	err = repo.SetGroupMember(ctx, privateChat.ID, adminID, memberID, true)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected private chat rejection, got %v", err)
	}
}

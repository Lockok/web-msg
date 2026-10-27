//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestChatRepository_ListChatMemberIDsIntegration(t *testing.T) {
	pool := newChatTestPool(t)
	repository := NewChatRepository(pool)

	aliceID := createChatTestUser(t, pool)
	bobID := createChatTestUser(t, pool)
	charlieID := createChatTestUser(t, pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	aliceBobChat, err := repository.GetOrCreatePrivateChat(
		ctx,
		aliceID,
		bobID,
	)
	if err != nil {
		t.Fatalf("create Alice-Bob chat: %v", err)
	}

	_, err = repository.GetOrCreatePrivateChat(
		ctx,
		aliceID,
		charlieID,
	)
	if err != nil {
		t.Fatalf("create Alice-Charlie chat: %v", err)
	}

	userIDs, err := repository.ListChatMemberIDs(ctx, aliceBobChat.ID)
	if err != nil {
		t.Fatalf("list chat members: %v", err)
	}

	if len(userIDs) != 2 {
		t.Fatalf("expected 2 members, got %d", len(userIDs))
	}

	found := make(map[uuid.UUID]bool)
	for _, userID := range userIDs {
		found[userID] = true
	}

	if !found[aliceID] || !found[bobID] {
		t.Fatal("expected Alice and Bob to be chat members")
	}

	if found[charlieID] {
		t.Fatal("member of another chat leaked into the result")
	}

	empty, err := repository.ListChatMemberIDs(ctx, uuid.New())
	if err != nil {
		t.Fatalf("list members of missing chat: %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatal("expected non-nil empty slice for missing chat")
	}
}

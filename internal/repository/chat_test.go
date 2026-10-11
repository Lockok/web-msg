package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestOrderUserIDs(t *testing.T) {
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		name string
		a    uuid.UUID
		b    uuid.UUID
	}{
		{"already ordered", low, high},
		{"reversed", high, low},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLow, gotHigh := orderUserIDs(tt.a, tt.b)

			if gotLow != low || gotHigh != high {
				t.Fatalf(
					"expected (%s, %s), got (%s, %s)",
					low,
					high,
					gotLow,
					gotHigh,
				)
			}
		})
	}
}

func TestChatRepository_GetOrCreatePrivateChatInvalidIDs(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name   string
		userID uuid.UUID
		peerID uuid.UUID
	}{
		{"empty user ID", uuid.Nil, userID},
		{"empty peer ID", userID, uuid.Nil},
		{"same user", userID, userID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Без pool: неверные ID должны отклоняться до обращения к БД.
			repository := &ChatRepository{}

			chat, err := repository.GetOrCreatePrivateChat(
				context.Background(),
				tt.userID,
				tt.peerID,
			)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if chat != nil {
				t.Fatal("expected no chat for invalid IDs")
			}
		})
	}
}

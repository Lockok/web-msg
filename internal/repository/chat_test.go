package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestMapPrivateChatPairError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMissing bool
	}{
		{
			name: "missing low user",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "private_chat_pairs_user_low_id_fkey",
			},
			wantMissing: true,
		},
		{
			name: "missing high user",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "private_chat_pairs_user_high_id_fkey",
			},
			wantMissing: true,
		},
		{
			name: "missing chat is not a missing user",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "private_chat_pairs_chat_id_fkey",
			},
		},
		{
			name: "unique conflict stays unchanged",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "private_chat_pairs_users_unique",
			},
		},
		{
			name: "connection error stays unchanged",
			err:  errors.New("database unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPrivateChatPairError(tt.err)

			if tt.wantMissing {
				if !errors.Is(got, domain.ErrUserNotFound) {
					t.Fatalf("expected ErrUserNotFound, got %v", got)
				}
				return
			}

			if !errors.Is(got, tt.err) {
				t.Fatalf("expected original error, got %v", got)
			}

			if errors.Is(got, domain.ErrUserNotFound) {
				t.Fatal("unrelated error must not become ErrUserNotFound")
			}
		})
	}
}

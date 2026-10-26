package realtime

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type mockChatMemberLister struct {
	userIDs   []uuid.UUID
	err       error
	calls     int
	gotChatID uuid.UUID
}

func (m *mockChatMemberLister) ListChatMemberIDs(
	ctx context.Context,
	chatID uuid.UUID,
) ([]uuid.UUID, error) {
	m.calls++
	m.gotChatID = chatID

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return m.userIDs, m.err
}

func TestMessageNotifier_NotifyMessageCreated(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	tests := []struct {
		name         string
		userIDs      []uuid.UUID
		repositoryErr error
		cancelContext bool
		wantErr      error
	}{
		{
			name:    "members are offline",
			userIDs: []uuid.UUID{uuid.New(), uuid.New()},
		},
		{
			name: "no members returned",
		},
		{
			name:          "member lookup failed",
			repositoryErr: databaseErr,
			wantErr:       databaseErr,
		},
		{
			name:          "context canceled",
			cancelContext: true,
			wantErr:       context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members := &mockChatMemberLister{
				userIDs: tt.userIDs,
				err:     tt.repositoryErr,
			}

			// Пустой Hub: активных соединений нет.
			notifier := NewMessageNotifier(members, NewHub())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.cancelContext {
				cancel()
			}

			message := domain.Message{
				ID:     uuid.New(),
				ChatID: uuid.New(),
				Text:   "hello",
				Seq:    1,
			}

			err := notifier.NotifyMessageCreated(ctx, message)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected notification error: %v", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}

			if members.calls != 1 {
				t.Fatalf(
					"expected one member lookup, got %d",
					members.calls,
				)
			}

			if members.gotChatID != message.ChatID {
				t.Fatalf(
					"expected chat %s, got %s",
					message.ChatID,
					members.gotChatID,
				)
			}
		})
	}
}
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

func TestMessageRepository_ListChatMessagesAfterPaginationIntegration(
	t *testing.T,
) {
	repository, _, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	saved := make(map[int64]uuid.UUID)

	for i := int64(1); i <= 5; i++ {
		message, err := repository.Send(ctx, domain.SendMessageInput{
			ChatID:          chatID,
			SenderID:        senderID,
			ClientMessageID: uuid.New(),
			Text:            "Привет",
		})
		if err != nil {
			t.Fatalf("send message %d: %v", i, err)
		}

		if message.Seq != i {
			t.Fatalf("expected sequence %d, got %d", i, message.Seq)
		}

		saved[i] = message.ID
	}

	tests := []struct {
		name     string
		afterSeq int64
		pageSize int32
		wantSeqs []int64
	}{
		{
			name:     "from beginning",
			afterSeq: 0,
			pageSize: 101,
			wantSeqs: []int64{1, 2, 3, 4, 5},
		},
		{
			name:     "strictly after cursor",
			afterSeq: 2,
			pageSize: 101,
			wantSeqs: []int64{3, 4, 5},
		},
		{
			name:     "limited page",
			afterSeq: 2,
			pageSize: 2,
			wantSeqs: []int64{3, 4},
		},
		{
			name:     "at latest message",
			afterSeq: 5,
			pageSize: 101,
		},
		{
			name:     "beyond latest message",
			afterSeq: 100,
			pageSize: 101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := repository.ListChatMessagesAfter(
				ctx,
				chatID,
				senderID,
				tt.afterSeq,
				tt.pageSize,
			)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}

			if messages == nil {
				t.Fatal("expected non-nil slice")
			}

			if len(messages) != len(tt.wantSeqs) {
				t.Fatalf(
					"expected %d messages, got %d",
					len(tt.wantSeqs),
					len(messages),
				)
			}

			for i, wantSeq := range tt.wantSeqs {
				got := messages[i]

				if got.Seq != wantSeq ||
					got.ID != saved[wantSeq] ||
					got.ChatID != chatID ||
					got.SenderID != senderID ||
					got.Text != "Привет" {
					t.Fatalf(
						"unexpected message at position %d: %+v",
						i,
						got,
					)
				}
			}
		})
	}
}

func TestMessageRepository_ListChatMessagesAfterAccessIntegration(
	t *testing.T,
) {
	tests := []struct {
		name    string
		wantErr error
	}{
		{
			name:    "missing chat",
			wantErr: domain.ErrChatNotFound,
		},
		{
			name:    "outsider",
			wantErr: domain.ErrNotChatMember,
		},
		{
			name:    "removed member",
			wantErr: domain.ErrNotChatMember,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, pool, chatID, senderID :=
				newMessageTestFixture(t)

			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			_, err := repository.Send(ctx, domain.SendMessageInput{
				ChatID:          chatID,
				SenderID:        senderID,
				ClientMessageID: uuid.New(),
				Text:            "private message",
			})
			if err != nil {
				t.Fatalf("send message: %v", err)
			}

			readerID := senderID

			switch tt.name {
			case "missing chat":
				chatID = uuid.New()

			case "outsider":
				readerID = createChatTestUser(t, pool)

			case "removed member":
				_, err := pool.Exec(
					ctx,
					`DELETE FROM chat_members
					 WHERE chat_id = $1 AND user_id = $2`,
					chatID,
					senderID,
				)
				if err != nil {
					t.Fatalf("remove member: %v", err)
				}
			}

			messages, err := repository.ListChatMessagesAfter(
				ctx,
				chatID,
				readerID,
				0,
				101,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"expected error %v, got %v",
					tt.wantErr,
					err,
				)
			}

			if messages != nil {
				t.Fatal("access failure must not return messages")
			}
		})
	}
}

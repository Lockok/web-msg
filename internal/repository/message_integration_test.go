//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newMessageTestFixture(
	t *testing.T,
) (*MessageRepository, *pgxpool.Pool, uuid.UUID, uuid.UUID) {
	t.Helper()

	pool := newChatTestPool(t)

	senderID := createChatTestUser(t, pool)
	peerID := createChatTestUser(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat, err := NewChatRepository(pool).GetOrCreatePrivateChat(
		ctx,
		senderID,
		peerID,
	)
	if err != nil {
		t.Fatalf("create message test chat: %v", err)
	}

	return NewMessageRepository(pool), pool, chat.ID, senderID
}

type messageTestState struct {
	count   int64
	lastSeq int64
}

func readMessageTestState(
	t *testing.T,
	pool *pgxpool.Pool,
	chatID uuid.UUID,
) messageTestState {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var state messageTestState

	err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM messages WHERE chat_id = c.id),
			c.last_message_seq
		FROM chats AS c
		WHERE c.id = $1`,
		chatID,
	).Scan(&state.count, &state.lastSeq)

	if err != nil {
		t.Fatalf("read message state: %v", err)
	}

	return state
}

func TestMessageRepository_SendIdempotencyIntegration(t *testing.T) {
	repository, pool, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := domain.SendMessageInput{
		ChatID:          chatID,
		SenderID:        senderID,
		ClientMessageID: uuid.New(),
		Text:            "  hello\nworld  ",
	}

	message, err := repository.Send(ctx, input)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if message == nil {
		t.Fatal("expected created message")
	}

	if message.ID == uuid.Nil ||
		message.ChatID != chatID ||
		message.SenderID != senderID ||
		message.ClientMessageID != input.ClientMessageID ||
		message.Text != input.Text ||
		message.Seq != 1 ||
		message.CreatedAt.IsZero() {
		t.Fatalf("unexpected message: %+v", message)
	}

	state := readMessageTestState(t, pool, chatID)

	if state != (messageTestState{count: 1, lastSeq: 1}) {
		t.Fatalf("unexpected state after creation: %+v", state)
	}

	// Проверяем реально сохранённый ID и текст.
	var storedID uuid.UUID
	var storedText string

	err = pool.QueryRow(ctx, `
		SELECT id, text
		FROM messages
		WHERE chat_id = $1 AND seq = 1`,
		chatID,
	).Scan(&storedID, &storedText)

	if err != nil {
		t.Fatalf("read stored message: %v", err)
	}

	if storedID != message.ID || storedText != input.Text {
		t.Fatal("stored message differs from the returned message")
	}

	// Тот же ключ и текст — то же сообщение.
	repeated, err := repository.Send(ctx, input)
	if err != nil {
		t.Fatalf("repeat message: %v", err)
	}

	if repeated == nil {
		t.Fatal("expected existing message")
	}

	if repeated.ID != message.ID ||
		repeated.Seq != message.Seq ||
		repeated.Text != message.Text ||
		!repeated.CreatedAt.Equal(message.CreatedAt) {
		t.Fatal("repeated request returned a different message")
	}

	if after := readMessageTestState(t, pool, chatID); after != state {
		t.Fatalf("repeated request changed state: %+v", after)
	}

	// Тот же ключ, но другой текст — конфликт.
	input.Text = "different text"

	conflicting, err := repository.Send(ctx, input)

	if !errors.Is(err, domain.ErrMessageConflict) {
		t.Fatalf("expected ErrMessageConflict, got %v", err)
	}

	if conflicting != nil {
		t.Fatal("must not return a message after a conflict")
	}

	if after := readMessageTestState(t, pool, chatID); after != state {
		t.Fatalf("conflicting request changed state: %+v", after)
	}
}

func TestMessageRepository_SendAccessIntegration(t *testing.T) {
	tests := []struct {
		name        string
		missingChat bool
		wantError   error
	}{
		{
			name:        "missing chat",
			missingChat: true,
			wantError:   domain.ErrChatNotFound,
		},
		{
			name:      "sender is not a member",
			wantError: domain.ErrNotChatMember,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, pool, chatID, senderID := newMessageTestFixture(t)

			before := readMessageTestState(t, pool, chatID)

			input := domain.SendMessageInput{
				ChatID:          chatID,
				SenderID:        senderID,
				ClientMessageID: uuid.New(),
				Text:            "not allowed",
			}

			if tt.missingChat {
				input.ChatID = uuid.New()
			} else {
				input.SenderID = createChatTestUser(t, pool)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			message, err := repository.Send(ctx, input)

			if !errors.Is(err, tt.wantError) {
				t.Fatalf("expected %v, got %v", tt.wantError, err)
			}

			if message != nil {
				t.Fatal("must not return a message after access failure")
			}

			after := readMessageTestState(t, pool, chatID)
			if after != before {
				t.Fatalf(
					"access failure changed state: before=%+v, after=%+v",
					before,
					after,
				)
			}
		})
	}
}

func TestMessageRepository_SendConcurrentIntegration(t *testing.T) {
	tests := []struct {
		name       string
		sameKey    bool
		wantStored int64
	}{
		{"distinct messages", false, 12},
		{"same message replay", true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, pool, chatID, senderID := newMessageTestFixture(t)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			const workers = 12

			type result struct {
				message          *domain.Message
				expectedClientID uuid.UUID
				err              error
			}

			start := make(chan struct{})
			results := make(chan result, workers)
			commonClientID := uuid.New()

			var wg sync.WaitGroup

			for i := 0; i < workers; i++ {
				clientID := commonClientID
				if !tt.sameKey {
					clientID = uuid.New()
				}

				input := domain.SendMessageInput{
					ChatID:          chatID,
					SenderID:        senderID,
					ClientMessageID: clientID,
					Text:            "concurrent message",
				}

				wg.Add(1)

				go func(input domain.SendMessageInput) {
					defer wg.Done()

					<-start

					message, err := repository.Send(ctx, input)

					results <- result{
						message:          message,
						expectedClientID: input.ClientMessageID,
						err:              err,
					}
				}(input)
			}

			close(start)
			wg.Wait()
			close(results)

			ids := make(map[uuid.UUID]struct{})
			sequences := make(map[int64]struct{})
			successful := 0

			for r := range results {
				if r.err != nil {
					t.Errorf("concurrent send failed: %v", r.err)
					continue
				}

				if r.message == nil {
					t.Error("expected a message")
					continue
				}

				message := r.message

				if message.ID == uuid.Nil ||
					message.ChatID != chatID ||
					message.SenderID != senderID ||
					message.ClientMessageID != r.expectedClientID ||
					message.Text != "concurrent message" {
					t.Errorf("unexpected message: %+v", message)
					continue
				}

				successful++
				ids[message.ID] = struct{}{}
				sequences[message.Seq] = struct{}{}
			}

			if successful != workers {
				t.Errorf("expected %d successful sends, got %d", workers, successful)
			}

			if int64(len(ids)) != tt.wantStored {
				t.Errorf("expected %d unique messages, got %d", tt.wantStored, len(ids))
			}

			if int64(len(sequences)) != tt.wantStored {
				t.Errorf(
					"expected %d unique sequences, got %d",
					tt.wantStored,
					len(sequences),
				)
			}

			for seq := int64(1); seq <= tt.wantStored; seq++ {
				if _, exists := sequences[seq]; !exists {
					t.Errorf("missing sequence %d", seq)
				}
			}

			state := readMessageTestState(t, pool, chatID)

			want := messageTestState{
				count:   tt.wantStored,
				lastSeq: tt.wantStored,
			}

			if state != want {
				t.Fatalf("expected state %+v, got %+v", want, state)
			}
		})
	}
}

func TestMessageRepository_SendInsertFailureRollbackIntegration(t *testing.T) {
	repository, pool, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := domain.SendMessageInput{
		ChatID:          chatID,
		SenderID:        senderID,
		ClientMessageID: uuid.New(),
		Text:            "first message",
	}

	first, err := repository.Send(ctx, input)
	if err != nil {
		t.Fatalf("send first message: %v", err)
	}

	if first == nil || first.Seq != 1 {
		t.Fatal("expected first message with sequence 1")
	}

	before := readMessageTestState(t, pool, chatID)

	input.ClientMessageID = uuid.New()
	input.Text = "invalid\x00text"

	message, err := repository.Send(ctx, input)
	if err == nil {
		t.Fatal("expected insertion failure")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "22021" {
		t.Fatalf("expected PostgreSQL invalid encoding error, got %v", err)
	}

	if message != nil {
		t.Fatal("must not return a message after insertion failure")
	}

	after := readMessageTestState(t, pool, chatID)
	if after != before {
		t.Fatalf(
			"insertion failure changed state: before=%+v, after=%+v",
			before,
			after,
		)
	}

	// Следующая успешная отправка получает 2, а не 3.
	input.ClientMessageID = uuid.New()
	input.Text = "second message"

	second, err := repository.Send(ctx, input)
	if err != nil {
		t.Fatalf("send after rollback: %v", err)
	}

	if second == nil || second.Seq != 2 {
		t.Fatal("expected sequence 2 after rollback")
	}

	want := messageTestState{count: 2, lastSeq: 2}

	if state := readMessageTestState(t, pool, chatID); state != want {
		t.Fatalf("expected state %+v, got %+v", want, state)
	}
}

func TestMessageRepository_SendWithResultIntegration(t *testing.T) {
	repository, _, chatID, senderID := newMessageTestFixture(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	input := domain.SendMessageInput{
		ChatID:          chatID,
		SenderID:        senderID,
		ClientMessageID: uuid.New(),
		Text:            "Привет",
	}

	first, err := repository.SendWithResult(ctx, input)
	if err != nil {
		t.Fatalf("send new message: %v", err)
	}

	if first == nil || first.Message == nil {
		t.Fatal("expected saved message")
	}

	if !first.Created {
		t.Fatal("first send must report Created=true")
	}

	repeated, err := repository.SendWithResult(ctx, input)
	if err != nil {
		t.Fatalf("repeat send: %v", err)
	}

	if repeated == nil || repeated.Message == nil {
		t.Fatal("expected existing message")
	}

	if repeated.Created {
		t.Fatal("repeated send must report Created=false")
	}

	if repeated.Message.ID != first.Message.ID ||
		repeated.Message.Seq != first.Message.Seq {
		t.Fatal("repeated send must return the same message")
	}
}

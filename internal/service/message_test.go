package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type mockMessageRepository struct {
	message            *domain.Message
	err                error
	calls              int
	gotInput           domain.SendMessageInput
	history            []domain.Message
	historyErr         error
	historyCalls       int
	gotHistoryChatID   uuid.UUID
	gotReaderID        uuid.UUID
	gotHistoryPageSize int32
	gotBeforeSeq       *int64
	created            bool
}

func (m *mockMessageRepository) SendWithResult(
	ctx context.Context,
	input domain.SendMessageInput,
) (*domain.SendMessageResult, error) {
	m.calls++
	m.gotInput = input

	if m.err != nil {
		return nil, m.err
	}

	return &domain.SendMessageResult{
		Message: m.message,
		Created: m.created,
	}, nil
}

func (m *mockMessageRepository) ListChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	pageSize int32,
	beforeSeq *int64,
) ([]domain.Message, error) {
	m.historyCalls++
	m.gotHistoryChatID = chatID
	m.gotReaderID = userID
	m.gotHistoryPageSize = pageSize
	m.gotBeforeSeq = beforeSeq

	return m.history, m.historyErr
}

func validSendMessageInput() domain.SendMessageInput {
	return domain.SendMessageInput{
		ChatID:          uuid.New(),
		SenderID:        uuid.New(),
		ClientMessageID: uuid.New(),
		Text:            "hello",
	}
}

func TestMessageService_SendValidInput(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"single character", "a"},
		{"preserve whitespace", "  hello\nworld\t "},
		{"Unicode", "Привет 👋"},
		{"maximum ASCII bytes", strings.Repeat("a", domain.MaxMessageTextBytes)},
		{"maximum Cyrillic bytes", strings.Repeat("я", domain.MaxMessageTextBytes/2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validSendMessageInput()
			input.Text = tt.text

			want := &domain.Message{
				ID:              uuid.New(),
				ChatID:          input.ChatID,
				SenderID:        input.SenderID,
				ClientMessageID: input.ClientMessageID,
				Text:            input.Text,
				Seq:             1,
			}

			repository := &mockMessageRepository{message: want}
			service := NewMessageService(repository)

			message, err := service.Send(context.Background(), input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if message != want {
				t.Fatal("expected the message returned by repository")
			}

			if repository.calls != 1 {
				t.Fatalf("expected one repository call, got %d", repository.calls)
			}

			if repository.gotInput != input {
				t.Fatal("service changed the input before passing it to repository")
			}
		})
	}
}

func TestMessageService_SendInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.SendMessageInput)
	}{
		{
			"empty chat ID",
			func(input *domain.SendMessageInput) {
				input.ChatID = uuid.Nil
			},
		},
		{
			"empty sender ID",
			func(input *domain.SendMessageInput) {
				input.SenderID = uuid.Nil
			},
		},
		{
			"empty client message ID",
			func(input *domain.SendMessageInput) {
				input.ClientMessageID = uuid.Nil
			},
		},
		{
			"empty text",
			func(input *domain.SendMessageInput) {
				input.Text = ""
			},
		},
		{
			"whitespace text",
			func(input *domain.SendMessageInput) {
				input.Text = " \n\t\u00a0"
			},
		},
		{
			"ASCII over byte limit",
			func(input *domain.SendMessageInput) {
				input.Text = strings.Repeat("a", domain.MaxMessageTextBytes+1)
			},
		},
		{
			"Unicode over byte limit",
			func(input *domain.SendMessageInput) {
				input.Text = strings.Repeat("я", domain.MaxMessageTextBytes/2+1)
			},
		},
		{
			"invalid UTF-8",
			func(input *domain.SendMessageInput) {
				input.Text = string([]byte{0xff})
			},
		},
		{
			"NUL in text",
			func(input *domain.SendMessageInput) {
				input.Text = "hello\x00world"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validSendMessageInput()
			tt.change(&input)

			repository := &mockMessageRepository{}
			service := NewMessageService(repository)

			message, err := service.Send(context.Background(), input)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if message != nil {
				t.Fatal("expected no message for invalid input")
			}

			if repository.calls != 0 {
				t.Fatal("repository must not be called for invalid input")
			}
		})
	}
}

func TestMessageService_SendRepositoryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"missing chat", domain.ErrChatNotFound},
		{"not a member", domain.ErrNotChatMember},
		{"idempotency conflict", domain.ErrMessageConflict},
		{"database failure", errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &mockMessageRepository{err: tt.err}
			service := NewMessageService(repository)

			message, err := service.Send(
				context.Background(),
				validSendMessageInput(),
			)

			if !errors.Is(err, tt.err) {
				t.Fatalf("expected original error, got %v", err)
			}

			if message != nil {
				t.Fatal("expected no message after repository failure")
			}

			if repository.calls != 1 {
				t.Fatalf("expected one repository call, got %d", repository.calls)
			}
		})
	}
}

func TestMessageService_SendWithResult(t *testing.T) {
	tests := []struct {
		name    string
		created bool
	}{
		{name: "new message", created: true},
		{name: "repeated message", created: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validSendMessageInput()

			message := &domain.Message{
				ID:              uuid.New(),
				ChatID:          input.ChatID,
				SenderID:        input.SenderID,
				ClientMessageID: input.ClientMessageID,
				Text:            input.Text,
				Seq:             1,
			}

			repository := &mockMessageRepository{
				message: message,
				created: tt.created,
			}
			service := NewMessageService(repository)

			result, err := service.SendWithResult(
				context.Background(),
				input,
			)
			if err != nil {
				t.Fatalf("send message: %v", err)
			}

			if result == nil || result.Message != message {
				t.Fatal("expected repository message")
			}

			if result.Created != tt.created {
				t.Fatalf(
					"expected Created=%v, got %v",
					tt.created,
					result.Created,
				)
			}

			if repository.calls != 1 {
				t.Fatalf(
					"expected one repository call, got %d",
					repository.calls,
				)
			}

			if repository.gotInput != input {
				t.Fatal("service changed the message input")
			}
		})
	}
}

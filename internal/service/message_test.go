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
	message  *domain.Message
	err      error
	calls    int
	gotInput domain.SendMessageInput
}

func (m *mockMessageRepository) Send(
	ctx context.Context,
	input domain.SendMessageInput,
) (*domain.Message, error) {
	m.calls++
	m.gotInput = input

	return m.message, m.err
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

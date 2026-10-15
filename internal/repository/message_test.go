package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageRepository_SendInvalidInput(t *testing.T) {
	valid := domain.SendMessageInput{
		ChatID:          uuid.New(),
		SenderID:        uuid.New(),
		ClientMessageID: uuid.New(),
		Text:            "hello",
	}

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
				input.Text = " \n\t "
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.change(&input)

			// Неверные данные должны отклоняться до обращения к БД.
			repository := &MessageRepository{}

			message, err := repository.Send(context.Background(), input)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if message != nil {
				t.Fatal("expected no message for invalid input")
			}
		})
	}
}

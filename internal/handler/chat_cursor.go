package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

const maxChatCursorLength = 512

type chatCursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func encodeChatCursor(cursor *domain.ChatCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}

	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return "", fmt.Errorf("%w: invalid chat cursor", domain.ErrInvalidInput)
	}

	payload := chatCursorPayload{
		CreatedAt: cursor.CreatedAt.UTC(),
		ID:        cursor.ID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: invalid chat cursor", domain.ErrInvalidInput)
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)

	if len(encoded) > maxChatCursorLength {
		return "", fmt.Errorf("%w: invalid chat cursor", domain.ErrInvalidInput)
	}

	return encoded, nil
}

func decodeChatCursor(encoded string) (*domain.ChatCursor, error) {
	if encoded == "" {
		return nil, nil
	}

	invalidErr := fmt.Errorf(
		"%w: invalid chat cursor",
		domain.ErrInvalidInput,
	)

	if len(encoded) > maxChatCursorLength {
		return nil, invalidErr
	}

	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, invalidErr
	}

	// Запрещаем альтернативные представления, включая переводы строк.
	if base64.RawURLEncoding.EncodeToString(data) != encoded {
		return nil, invalidErr
	}

	var payload chatCursorPayload

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return nil, invalidErr
	}

	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, invalidErr
	}

	if payload.ID == uuid.Nil || payload.CreatedAt.IsZero() {
		return nil, invalidErr
	}

	return &domain.ChatCursor{
		CreatedAt: payload.CreatedAt.UTC(),
		ID:        payload.ID,
	}, nil
}

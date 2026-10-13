package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestChatCursorRoundTrip(t *testing.T) {
	want := &domain.ChatCursor{
		ID: uuid.New(),
		CreatedAt: time.Date(
			2026, 10, 11, 15, 0, 0, 123456000,
			time.FixedZone("UTC+3", 3*60*60),
		),
	}

	encoded, err := encodeChatCursor(want)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	if encoded == "" || strings.ContainsAny(encoded, "+/=") {
		t.Fatal("expected non-empty URL-safe Base64 without padding")
	}

	got, err := decodeChatCursor(encoded)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}

	if got == nil {
		t.Fatal("expected decoded cursor")
	}

	if got.ID != want.ID || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("cursor changed after round trip: %+v", got)
	}

	if got.CreatedAt.Location() != time.UTC {
		t.Fatal("expected timestamp normalized to UTC")
	}
}

func TestChatCursorAbsent(t *testing.T) {
	encoded, err := encodeChatCursor(nil)
	if err != nil || encoded != "" {
		t.Fatalf("expected empty encoded cursor, got %q, %v", encoded, err)
	}

	cursor, err := decodeChatCursor("")
	if err != nil || cursor != nil {
		t.Fatalf("expected absent cursor, got %+v, %v", cursor, err)
	}
}

func TestDecodeChatCursorInvalid(t *testing.T) {
	encode := func(payload string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(payload))
	}

	id := uuid.New().String()
	validPayload := fmt.Sprintf(
		`{"created_at":"2026-10-11T12:00:00.123456Z","id":%q}`,
		id,
	)
	validEncoded := encode(validPayload)

	tests := []struct {
		name  string
		value string
	}{
		{"invalid Base64", "!not-base64!"},
		{"too long", strings.Repeat("A", maxChatCursorLength+1)},
		{"padding is forbidden", validEncoded + "="},
		{"newline is forbidden", validEncoded + "\n"},
		{"malformed JSON", encode(`{"id":`)},
		{"empty object", encode(`{}`)},
		{"null payload", encode(`null`)},
		{
			"missing timestamp",
			encode(fmt.Sprintf(`{"id":%q}`, id)),
		},
		{
			"missing ID",
			encode(`{"created_at":"2026-10-11T12:00:00Z"}`),
		},
		{
			"invalid timestamp",
			encode(fmt.Sprintf(`{"created_at":"bad-time","id":%q}`, id)),
		},
		{
			"invalid UUID",
			encode(`{"created_at":"2026-10-11T12:00:00Z","id":"bad-id"}`),
		},
		{
			"nil UUID",
			encode(fmt.Sprintf(
				`{"created_at":"2026-10-11T12:00:00Z","id":%q}`,
				uuid.Nil.String(),
			)),
		},
		{
			"zero timestamp",
			encode(fmt.Sprintf(
				`{"created_at":"0001-01-01T00:00:00Z","id":%q}`,
				id,
			)),
		},
		{
			"unknown field",
			encode(fmt.Sprintf(
				`{"created_at":"2026-10-11T12:00:00Z","id":%q,"extra":true}`,
				id,
			)),
		},
		{"second JSON value", encode(validPayload + ` {}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor, err := decodeChatCursor(tt.value)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if cursor != nil {
				t.Fatal("expected no cursor for invalid input")
			}
		})
	}
}

func TestEncodeChatCursorInvalid(t *testing.T) {
	tests := []struct {
		name   string
		cursor *domain.ChatCursor
	}{
		{
			"missing ID",
			&domain.ChatCursor{CreatedAt: time.Now().UTC()},
		},
		{
			"missing timestamp",
			&domain.ChatCursor{ID: uuid.New()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeChatCursor(tt.cursor)

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}

			if encoded != "" {
				t.Fatal("must not encode an invalid cursor")
			}
		})
	}
}

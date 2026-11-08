package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

// Реализация для существующих тестов других методов ChatHandler.
func (m *mockChatService) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	return nil, errors.New("unexpected CreateGroupChat call")
}

type groupChatServiceStub struct {
	*mockChatService
	calls int
	input domain.CreateGroupChatInput
	chat  *domain.Chat
	err   error
}

func (m *groupChatServiceStub) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	m.calls++
	m.input = input
	return m.chat, m.err
}

func (m *mockChatService) ClearChatHistory(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (int64, error) {
	return 0, errors.New("unexpected ClearChatHistory call")
}

func (m *mockChatService) SetGroupMember(
	ctx context.Context,
	chatID, actorID, targetID uuid.UUID,
	add bool,
) error {
	return errors.New("unexpected SetGroupMember call")
}

func TestChatHandler_CreateGroupChat(t *testing.T) {
	userID := uuid.New()
	want := &domain.Chat{ID: uuid.New(), Type: "group", Name: "Команда"}

	svc := &groupChatServiceStub{chat: want}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewChatHandler(svc, logger)
	jwtManager := auth.NewJWTManager("test-secret")

	token, err := jwtManager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	endpoint := middleware.Auth(jwtManager)(
		http.HandlerFunc(handler.CreateGroupChat),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chats/group",
		strings.NewReader(`{"name":"Команда"}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var response ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != want.ID || response.Type != "group" || response.Name != want.Name {
		t.Fatalf("unexpected response: %+v", response)
	}
	if svc.calls != 1 || svc.input.CreatorID != userID || svc.input.Name != want.Name {
		t.Fatalf("unexpected service call: calls=%d input=%+v", svc.calls, svc.input)
	}
}

func TestChatHandler_CreateGroupChatErrors(t *testing.T) {
	internalErr := errors.New("sensitive database error")

	tests := []struct {
		name       string
		body       string
		noAuth     bool
		serviceErr error
		status     int
		calls      int
	}{
		{"unauthorized", `{"name":"group"}`, true, nil, 401, 0},
		{"invalid JSON", `{"name":`, false, nil, 400, 0},
		{"extra creator", `{"name":"group","creator_id":"fake"}`, false, nil, 400, 0},
		{"invalid name", `{"name":""}`, false, domain.ErrInvalidInput, 400, 1},
		{"missing user", `{"name":"group"}`, false, domain.ErrUserNotFound, 404, 1},
		{"internal error", `{"name":"group"}`, false, internalErr, 500, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &groupChatServiceStub{err: tt.serviceErr}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewChatHandler(svc, logger)
			jwtManager := auth.NewJWTManager("test-secret")

			token, err := jwtManager.Generate(uuid.New())
			if err != nil {
				t.Fatalf("generate token: %v", err)
			}

			endpoint := middleware.Auth(jwtManager)(
				http.HandlerFunc(handler.CreateGroupChat),
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/chats/group",
				strings.NewReader(tt.body),
			)
			if !tt.noAuth {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rec := httptest.NewRecorder()
			endpoint.ServeHTTP(rec, req)

			if rec.Code != tt.status || svc.calls != tt.calls {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, svc.calls, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), internalErr.Error()) {
				t.Fatal("response exposes internal error")
			}
		})
	}
}

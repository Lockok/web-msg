package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

type mockChatService struct {
	chat      *domain.Chat
	err       error
	calls     int
	gotUserID uuid.UUID
	gotPeerID uuid.UUID
}

func (m *mockChatService) GetOrCreatePrivateChat(
	ctx context.Context,
	userID uuid.UUID,
	peerID uuid.UUID,
) (*domain.Chat, error) {
	m.calls++
	m.gotUserID = userID
	m.gotPeerID = peerID

	return m.chat, m.err
}

func TestChatHandler_GetOrCreatePrivateChat(t *testing.T) {
	userID := uuid.New()
	peerID := uuid.New()

	want := &domain.Chat{
		ID:        uuid.New(),
		Type:      "private",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	service := &mockChatService{chat: want}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := NewChatHandler(service, log)
	jwtManager := auth.NewJWTManager("test-secret")

	token, err := jwtManager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	endpoint := middleware.Auth(jwtManager)(
		http.HandlerFunc(handler.GetOrCreatePrivateChat),
	)

	body := fmt.Sprintf(`{"peer_id":%q}`, peerID.String())

	req := httptest.NewRequest(
		http.MethodPost,
		"/chats/private",
		strings.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected JSON response")
	}

	var response ChatResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != want.ID ||
		response.Type != want.Type ||
		!response.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("unexpected response: %+v", response)
	}

	if service.calls != 1 {
		t.Fatalf("expected one service call, got %d", service.calls)
	}

	if service.gotUserID != userID || service.gotPeerID != peerID {
		t.Fatal("service received incorrect user IDs")
	}
}

func TestChatHandler_GetOrCreatePrivateChatErrors(t *testing.T) {
	userID := uuid.New()
	peerID := uuid.New()

	validBody := fmt.Sprintf(`{"peer_id":%q}`, peerID.String())
	dbErr := errors.New("sensitive database error")

	tests := []struct {
		name        string
		body        string
		withoutAuth bool
		invalidJWT  bool
		serviceErr  error
		wantStatus  int
		wantCalls   int
	}{
		{
			name:        "missing authorization",
			body:        validBody,
			withoutAuth: true,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			body:       validBody,
			invalidJWT: true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed JSON",
			body:       `{"peer_id":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing peer ID",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid peer ID",
			body:       `{"peer_id":"not-a-uuid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil peer ID",
			body:       fmt.Sprintf(`{"peer_id":%q}`, uuid.Nil.String()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "attempt to supply current user ID",
			body: fmt.Sprintf(
				`{"peer_id":%q,"user_id":%q}`,
				peerID.String(),
				uuid.New().String(),
			),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "second JSON value",
			body:       validBody + ` {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversized body",
			body:       validBody + strings.Repeat(" ", int(maxJSONBodyBytes)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "chat with yourself",
			body:       fmt.Sprintf(`{"peer_id":%q}`, userID.String()),
			serviceErr: fmt.Errorf("service: %w", domain.ErrInvalidInput),
			wantStatus: http.StatusBadRequest,
			wantCalls:  1,
		},
		{
			name:       "missing user",
			body:       validBody,
			serviceErr: fmt.Errorf("service: %w", domain.ErrUserNotFound),
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name:       "internal error",
			body:       validBody,
			serviceErr: dbErr,
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockChatService{err: tt.serviceErr}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))

			handler := NewChatHandler(service, log)
			jwtManager := auth.NewJWTManager("test-secret")

			token, err := jwtManager.Generate(userID)
			if err != nil {
				t.Fatalf("generate token: %v", err)
			}

			endpoint := middleware.Auth(jwtManager)(
				http.HandlerFunc(handler.GetOrCreatePrivateChat),
			)

			req := httptest.NewRequest(
				http.MethodPost,
				"/chats/private",
				strings.NewReader(tt.body),
			)

			if !tt.withoutAuth {
				if tt.invalidJWT {
					token = "invalid-token"
				}

				req.Header.Set("Authorization", "Bearer "+token)
			}

			rec := httptest.NewRecorder()
			endpoint.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"expected status %d, got %d: %s",
					tt.wantStatus,
					rec.Code,
					rec.Body.String(),
				)
			}

			if service.calls != tt.wantCalls {
				t.Fatalf(
					"expected %d service calls, got %d",
					tt.wantCalls,
					service.calls,
				)
			}

			if strings.Contains(rec.Body.String(), dbErr.Error()) {
				t.Fatal("response exposes an internal error")
			}
		})
	}
}

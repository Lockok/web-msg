package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

func newChatListTestEndpoint(
	t *testing.T,
	service *mockChatService,
) (http.Handler, string, uuid.UUID) {
	t.Helper()

	userID := uuid.New()
	jwtManager := auth.NewJWTManager("test-secret")

	token, err := jwtManager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewChatHandler(service, log)

	endpoint := middleware.Auth(jwtManager)(
		http.HandlerFunc(handler.ListUserChats),
	)

	return endpoint, token, userID
}

func TestChatHandler_ListUserChats(t *testing.T) {
	stamp := time.Date(2026, 10, 11, 12, 0, 0, 123456000, time.UTC)

	inputCursor := &domain.ChatCursor{
		ID:        uuid.New(),
		CreatedAt: stamp.Add(time.Minute),
	}

	chat := domain.Chat{
		ID:        uuid.New(),
		Type:      "private",
		CreatedAt: stamp,
	}

	nextCursor := &domain.ChatCursor{
		ID:        chat.ID,
		CreatedAt: chat.CreatedAt,
	}

	encodedInput, err := encodeChatCursor(inputCursor)
	if err != nil {
		t.Fatalf("encode input cursor: %v", err)
	}

	service := &mockChatService{
		page: &domain.ChatPage{
			Items:      []domain.Chat{chat},
			NextCursor: nextCursor,
		},
	}

	endpoint, token, userID := newChatListTestEndpoint(t, service)

	query := url.Values{
		"limit":  {"1"},
		"cursor": {encodedInput},
	}

	req := httptest.NewRequest(http.MethodGet, "/chats?"+query.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected JSON response")
	}

	var response ChatListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Items) != 1 {
		t.Fatalf("expected one chat, got %d", len(response.Items))
	}

	item := response.Items[0]
	if item.ID != chat.ID ||
		item.Type != chat.Type ||
		!item.CreatedAt.Equal(chat.CreatedAt) {
		t.Fatal("unexpected chat in response")
	}

	if response.NextCursor == nil {
		t.Fatal("expected next cursor")
	}

	decodedNext, err := decodeChatCursor(*response.NextCursor)
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}

	if decodedNext.ID != nextCursor.ID ||
		!decodedNext.CreatedAt.Equal(nextCursor.CreatedAt) {
		t.Fatal("unexpected next cursor")
	}

	if service.listCalls != 1 ||
		service.gotListUserID != userID ||
		service.gotLimit != 1 {
		t.Fatal("service received unexpected arguments or call count")
	}

	if service.gotCursor == nil ||
		service.gotCursor.ID != inputCursor.ID ||
		!service.gotCursor.CreatedAt.Equal(inputCursor.CreatedAt) {
		t.Fatal("service received incorrect cursor")
	}
}

func TestChatHandler_ListUserChatsEmpty(t *testing.T) {
	service := &mockChatService{
		page: &domain.ChatPage{Items: []domain.Chat{}},
	}

	endpoint, token, userID := newChatListTestEndpoint(t, service)

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if string(fields["items"]) != "[]" {
		t.Fatal("expected items to be an empty array")
	}

	if string(fields["next_cursor"]) != "null" {
		t.Fatal("expected next_cursor to be null")
	}

	if service.listCalls != 1 ||
		service.gotListUserID != userID ||
		service.gotLimit != 20 ||
		service.gotCursor != nil {
		t.Fatal("unexpected default arguments")
	}
}

func TestChatHandler_ListUserChatsErrors(t *testing.T) {
	dbErr := errors.New("sensitive database error")

	tests := []struct {
		name        string
		query       string
		withoutAuth bool
		serviceErr  error
		page        *domain.ChatPage
		wantStatus  int
		wantCalls   int
	}{
		{
			name:        "missing authorization",
			withoutAuth: true,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:       "empty limit",
			query:      "?limit=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-numeric limit",
			query:      "?limit=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero limit",
			query:      "?limit=0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative limit",
			query:      "?limit=-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "limit over maximum",
			query:      "?limit=101",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate limit",
			query:      "?limit=1&limit=2",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate cursor",
			query:      "?cursor=a&cursor=b",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid cursor",
			query:      "?cursor=not-a-valid-cursor",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed query encoding",
			query:      "?cursor=%ZZ",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service rejects input",
			serviceErr: fmt.Errorf("service: %w", domain.ErrInvalidInput),
			wantStatus: http.StatusBadRequest,
			wantCalls:  1,
		},
		{
			name:       "database failure",
			serviceErr: dbErr,
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
		{
			name: "invalid cursor returned by service",
			page: &domain.ChatPage{
				Items:      []domain.Chat{},
				NextCursor: &domain.ChatCursor{},
			},
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockChatService{
				page:    tt.page,
				listErr: tt.serviceErr,
			}

			endpoint, token, _ := newChatListTestEndpoint(t, service)

			req := httptest.NewRequest(http.MethodGet, "/chats"+tt.query, nil)
			if !tt.withoutAuth {
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

			if service.listCalls != tt.wantCalls {
				t.Fatalf(
					"expected %d service calls, got %d",
					tt.wantCalls,
					service.listCalls,
				)
			}

			if strings.Contains(rec.Body.String(), dbErr.Error()) {
				t.Fatal("response exposes an internal error")
			}
		})
	}
}

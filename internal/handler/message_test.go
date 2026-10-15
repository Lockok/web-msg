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
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type mockMessageService struct {
	message  *domain.Message
	err      error
	calls    int
	gotInput domain.SendMessageInput

	historyPage      *domain.MessagePage
	historyErr       error
	historyCalls     int
	gotHistoryChatID uuid.UUID
	gotReaderID      uuid.UUID
	gotHistoryLimit  int
	gotBeforeSeq     *int64
}

func (m *mockMessageService) Send(
	ctx context.Context,
	input domain.SendMessageInput,
) (*domain.Message, error) {
	m.calls++
	m.gotInput = input

	return m.message, m.err
}

func (m *mockMessageService) ListChatMessages(
	ctx context.Context,
	chatID uuid.UUID,
	userID uuid.UUID,
	limit int,
	beforeSeq *int64,
) (*domain.MessagePage, error) {
	m.historyCalls++
	m.gotHistoryChatID = chatID
	m.gotReaderID = userID
	m.gotHistoryLimit = limit
	m.gotBeforeSeq = beforeSeq

	return m.historyPage, m.historyErr
}

func newMessageTestRouter(
	t *testing.T,
	service *mockMessageService,
	senderID uuid.UUID,
) (http.Handler, string) {
	t.Helper()

	jwtManager := auth.NewJWTManager("test-secret")

	token, err := jwtManager.Generate(senderID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewMessageHandler(service, log)

	router := chi.NewRouter()
	router.With(middleware.Auth(jwtManager)).Post(
		"/chats/{chat_id}/messages",
		handler.Send,
	)

	router.With(middleware.Auth(jwtManager)).Get(
		"/chats/{chat_id}/messages",
		handler.ListChatMessages,
	)

	return router, token
}

func TestMessageHandler_Send(t *testing.T) {
	texts := []string{
		"  hello\nworld  ",
		strings.Repeat("a", domain.MaxMessageTextBytes),
	}

	for _, text := range texts {
		t.Run(fmt.Sprintf("text_bytes_%d", len(text)), func(t *testing.T) {
			senderID := uuid.New()
			chatID := uuid.New()
			clientID := uuid.New()

			want := &domain.Message{
				ID:              uuid.New(),
				ChatID:          chatID,
				SenderID:        senderID,
				ClientMessageID: clientID,
				Text:            text,
				Seq:             1,
				CreatedAt:       time.Now().UTC().Truncate(time.Microsecond),
			}

			service := &mockMessageService{message: want}
			router, token := newMessageTestRouter(t, service, senderID)

			body, err := json.Marshal(SendMessageRequest{
				ClientMessageID: clientID.String(),
				Text:            text,
			})
			if err != nil {
				t.Fatalf("encode request: %v", err)
			}

			req := httptest.NewRequest(
				http.MethodPost,
				"/chats/"+chatID.String()+"/messages",
				strings.NewReader(string(body)),
			)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatal("expected JSON response")
			}

			var response MessageResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.ID != want.ID ||
				response.ChatID != chatID ||
				response.SenderID != senderID ||
				response.ClientMessageID != clientID ||
				response.Text != text ||
				response.Seq != 1 ||
				!response.CreatedAt.Equal(want.CreatedAt) {
				t.Fatalf("unexpected response: %+v", response)
			}

			expectedInput := domain.SendMessageInput{
				ChatID:          chatID,
				SenderID:        senderID,
				ClientMessageID: clientID,
				Text:            text,
			}

			if service.calls != 1 || service.gotInput != expectedInput {
				t.Fatal("service received unexpected input or call count")
			}
		})
	}
}

func TestMessageHandler_SendErrors(t *testing.T) {
	chatID := uuid.New()
	clientID := uuid.New()

	validBody := fmt.Sprintf(
		`{"client_message_id":%q,"text":"hello"}`,
		clientID.String(),
	)

	dbErr := errors.New("sensitive database error")

	tests := []struct {
		name        string
		chatID      string
		body        string
		withoutAuth bool
		serviceErr  error
		wantStatus  int
		wantCalls   int
	}{
		{
			name:        "missing authorization",
			chatID:      chatID.String(),
			body:        validBody,
			withoutAuth: true,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:       "invalid chat ID",
			chatID:     "bad-id",
			body:       validBody,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil chat ID",
			chatID:     uuid.Nil.String(),
			body:       validBody,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON",
			chatID:     chatID.String(),
			body:       `{"text":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing client ID",
			chatID:     chatID.String(),
			body:       `{"text":"hello"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid client ID",
			chatID:     chatID.String(),
			body:       `{"client_message_id":"bad-id","text":"hello"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil client ID",
			chatID:     chatID.String(),
			body:       fmt.Sprintf(`{"client_message_id":%q,"text":"hello"}`, uuid.Nil.String()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "attempt to supply sender",
			chatID: chatID.String(),
			body: fmt.Sprintf(
				`{"client_message_id":%q,"text":"hello","sender_id":%q}`,
				clientID.String(),
				uuid.New().String(),
			),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "second JSON value",
			chatID:     chatID.String(),
			body:       validBody + ` {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversized body",
			chatID:     chatID.String(),
			body:       validBody + strings.Repeat(" ", int(maxMessageBodyBytes)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "invalid text",
			chatID:     chatID.String(),
			body:       validBody,
			serviceErr: fmt.Errorf("service: %w", domain.ErrInvalidInput),
			wantStatus: http.StatusBadRequest,
			wantCalls:  1,
		},
		{
			name:       "not a member",
			chatID:     chatID.String(),
			body:       validBody,
			serviceErr: fmt.Errorf("service: %w", domain.ErrNotChatMember),
			wantStatus: http.StatusForbidden,
			wantCalls:  1,
		},
		{
			name:       "missing chat",
			chatID:     chatID.String(),
			body:       validBody,
			serviceErr: fmt.Errorf("service: %w", domain.ErrChatNotFound),
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name:       "idempotency conflict",
			chatID:     chatID.String(),
			body:       validBody,
			serviceErr: fmt.Errorf("service: %w", domain.ErrMessageConflict),
			wantStatus: http.StatusConflict,
			wantCalls:  1,
		},
		{
			name:       "internal failure",
			chatID:     chatID.String(),
			body:       validBody,
			serviceErr: dbErr,
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockMessageService{err: tt.serviceErr}
			router, token := newMessageTestRouter(t, service, uuid.New())

			req := httptest.NewRequest(
				http.MethodPost,
				"/chats/"+tt.chatID+"/messages",
				strings.NewReader(tt.body),
			)
			if !tt.withoutAuth {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"expected %d, got %d: %s",
					tt.wantStatus,
					rec.Code,
					rec.Body.String(),
				)
			}

			if service.calls != tt.wantCalls {
				t.Fatalf("expected %d calls, got %d", tt.wantCalls, service.calls)
			}

			if strings.Contains(rec.Body.String(), dbErr.Error()) {
				t.Fatal("response exposes an internal error")
			}
		})
	}
}

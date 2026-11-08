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

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type clearChatServiceStub struct {
	*mockChatService
	calls  int
	chatID uuid.UUID
	userID uuid.UUID
	cutoff int64
	err    error
}

func (s *clearChatServiceStub) ClearChatHistory(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (int64, error) {
	s.calls++
	s.chatID = chatID
	s.userID = userID
	return s.cutoff, s.err
}

func TestChatHandler_ClearChatHistory(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	internalErr := errors.New("sensitive database error")

	tests := []struct {
		name       string
		chatID     string
		noAuth     bool
		serviceErr error
		wantStatus int
		wantCalls  int
	}{
		{"success", chatID.String(), false, nil, 200, 1},
		{"unauthorized", chatID.String(), true, nil, 401, 0},
		{"invalid chat", "not-a-uuid", false, nil, 400, 0},
		{"nil chat", uuid.Nil.String(), false, nil, 400, 0},
		{"invalid input", chatID.String(), false, domain.ErrInvalidInput, 400, 1},
		{"not member", chatID.String(), false, domain.ErrNotChatMember, 403, 1},
		{"chat not found", chatID.String(), false, domain.ErrChatNotFound, 404, 1},
		{"internal error", chatID.String(), false, internalErr, 500, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &clearChatServiceStub{
				cutoff: 42,
				err:    tt.serviceErr,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewChatHandler(svc, logger)
			jwtManager := auth.NewJWTManager("test-secret")

			token, err := jwtManager.Generate(userID)
			if err != nil {
				t.Fatalf("generate token: %v", err)
			}

			router := chi.NewRouter()
			router.With(middleware.Auth(jwtManager)).Post(
				"/chats/{chat_id}/history/clear",
				handler.ClearChatHistory,
			)

			req := httptest.NewRequest(
				http.MethodPost,
				fmt.Sprintf("/chats/%s/history/clear", tt.chatID),
				nil,
			)
			if !tt.noAuth {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || svc.calls != tt.wantCalls {
				t.Fatalf(
					"status=%d calls=%d body=%s",
					rec.Code, svc.calls, rec.Body.String(),
				)
			}
			if strings.Contains(rec.Body.String(), internalErr.Error()) {
				t.Fatal("response exposes internal error")
			}

			if tt.wantCalls == 1 {
				if svc.chatID != chatID || svc.userID != userID {
					t.Fatalf(
						"unexpected arguments: chat=%s user=%s",
						svc.chatID, svc.userID,
					)
				}
			}

			if tt.wantStatus == http.StatusOK {
				var body struct {
					ClearedThroughSeq int64 `json:"cleared_through_seq"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if body.ClearedThroughSeq != 42 {
					t.Fatalf("unexpected cutoff: %d", body.ClearedThroughSeq)
				}
			}
		})
	}
}

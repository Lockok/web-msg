package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageHandler_ListChatMessages(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	nextSeq := int64(4)

	message := domain.Message{
		ID:              uuid.New(),
		ChatID:          chatID,
		SenderID:        uuid.New(),
		ClientMessageID: uuid.New(),
		Text:            "Привет",
		Seq:             4,
		CreatedAt:       time.Now().UTC().Truncate(time.Microsecond),
	}

	tests := []struct {
		name          string
		query         string
		page          *domain.MessagePage
		wantLimit     int
		wantBeforeSeq int64
	}{
		{
			name:      "defaults and empty history",
			page:      &domain.MessagePage{},
			wantLimit: 20,
		},
		{
			name:  "explicit pagination",
			query: "?limit=2&before_seq=6",
			page: &domain.MessagePage{
				Items:         []domain.Message{message},
				NextBeforeSeq: &nextSeq,
			},
			wantLimit:     2,
			wantBeforeSeq: 6,
		},
		{
			name:      "maximum limit",
			query:     "?limit=100",
			page:      &domain.MessagePage{},
			wantLimit: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockMessageService{historyPage: tt.page}
			router, token := newMessageTestRouter(t, service, userID)

			req := httptest.NewRequest(
				http.MethodGet,
				"/chats/"+chatID.String()+"/messages"+tt.query,
				nil,
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

			var response MessageHistoryResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.Items == nil ||
				len(response.Items) != len(tt.page.Items) {
				t.Fatal("unexpected items array")
			}

			for i, item := range response.Items {
				want := tt.page.Items[i]

				if item.ID != want.ID ||
					item.ChatID != want.ChatID ||
					item.SenderID != want.SenderID ||
					item.ClientMessageID != want.ClientMessageID ||
					item.Text != want.Text ||
					item.Seq != want.Seq ||
					!item.CreatedAt.Equal(want.CreatedAt) {
					t.Fatalf("unexpected message at position %d", i)
				}
			}

			if tt.page.NextBeforeSeq == nil {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
					t.Fatalf("decode response fields: %v", err)
				}

				if string(fields["next_before_seq"]) != "null" {
					t.Fatal("expected explicit null continuation")
				}
			} else if response.NextBeforeSeq == nil ||
				*response.NextBeforeSeq != *tt.page.NextBeforeSeq {
				t.Fatal("unexpected continuation cursor")
			}

			if service.historyCalls != 1 ||
				service.gotHistoryChatID != chatID ||
				service.gotReaderID != userID ||
				service.gotHistoryLimit != tt.wantLimit {
				t.Fatal("service received unexpected arguments or call count")
			}

			if tt.wantBeforeSeq == 0 {
				if service.gotBeforeSeq != nil {
					t.Fatal("expected no input cursor")
				}
			} else if service.gotBeforeSeq == nil ||
				*service.gotBeforeSeq != tt.wantBeforeSeq {
				t.Fatal("service received incorrect before_seq")
			}
		})
	}
}

func TestMessageHandler_ListChatMessagesErrors(t *testing.T) {
	chatID := uuid.New().String()
	dbErr := errors.New("sensitive database error")

	tests := []struct {
		name        string
		chatID      string
		query       string
		withoutAuth bool
		serviceErr  error
		wantStatus  int
		wantCalls   int
	}{
		{
			name:        "missing authorization",
			chatID:      chatID,
			withoutAuth: true,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:       "invalid chat ID",
			chatID:     "bad-id",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil chat ID",
			chatID:     uuid.Nil.String(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty limit",
			chatID:     chatID,
			query:      "?limit=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid limit",
			chatID:     chatID,
			query:      "?limit=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero limit",
			chatID:     chatID,
			query:      "?limit=0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "limit over maximum",
			chatID:     chatID,
			query:      "?limit=101",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate limit",
			chatID:     chatID,
			query:      "?limit=1&limit=2",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty before_seq",
			chatID:     chatID,
			query:      "?before_seq=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid before_seq",
			chatID:     chatID,
			query:      "?before_seq=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero before_seq",
			chatID:     chatID,
			query:      "?before_seq=0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative before_seq",
			chatID:     chatID,
			query:      "?before_seq=-1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "overflow before_seq",
			chatID:     chatID,
			query:      "?before_seq=9223372036854775808",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate before_seq",
			chatID:     chatID,
			query:      "?before_seq=4&before_seq=5",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed query",
			chatID:     chatID,
			query:      "?before_seq=%ZZ",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service rejects input",
			chatID:     chatID,
			serviceErr: fmt.Errorf("service: %w", domain.ErrInvalidInput),
			wantStatus: http.StatusBadRequest,
			wantCalls:  1,
		},
		{
			name:       "not a member",
			chatID:     chatID,
			serviceErr: fmt.Errorf("service: %w", domain.ErrNotChatMember),
			wantStatus: http.StatusForbidden,
			wantCalls:  1,
		},
		{
			name:       "missing chat",
			chatID:     chatID,
			serviceErr: fmt.Errorf("service: %w", domain.ErrChatNotFound),
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name:       "internal failure",
			chatID:     chatID,
			serviceErr: dbErr,
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockMessageService{historyErr: tt.serviceErr}
			router, token := newMessageTestRouter(t, service, uuid.New())

			req := httptest.NewRequest(
				http.MethodGet,
				"/chats/"+tt.chatID+"/messages"+tt.query,
				nil,
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

			if service.historyCalls != tt.wantCalls {
				t.Fatalf("expected %d calls, got %d", tt.wantCalls, service.historyCalls)
			}

			if strings.Contains(rec.Body.String(), dbErr.Error()) {
				t.Fatal("response exposes an internal error")
			}
		})
	}
}

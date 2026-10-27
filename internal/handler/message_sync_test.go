package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

func TestMessageHandler_SyncChatMessages(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	basePath := "/chats/" + chatID.String() + "/messages/sync"

	tests := []struct {
		name      string
		query     string
		wantAfter int64
		wantLimit int
		empty     bool
	}{
		{
			name:      "default limit and empty result",
			query:     "?after_seq=0",
			wantAfter: 0,
			wantLimit: 100,
			empty:     true,
		},
		{
			name:      "explicit limit and continuation",
			query:     "?after_seq=7&limit=2",
			wantAfter: 7,
			wantLimit: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &domain.MessageSyncPage{}

			if !tt.empty {
				page.Items = []domain.Message{
					{
						ID:              uuid.New(),
						ChatID:          chatID,
						SenderID:        userID,
						ClientMessageID: uuid.New(),
						Text:            "Привет 👋",
						Seq:             8,
					},
					{
						ID:              uuid.New(),
						ChatID:          chatID,
						SenderID:        userID,
						ClientMessageID: uuid.New(),
						Text:            "Вторая строка\nсообщения",
						Seq:             9,
					},
				}

				next := int64(9)
				page.NextAfterSeq = &next
			}

			service := &mockMessageService{syncPage: page}
			router, token := newMessageTestRouter(t, service, userID)

			req := httptest.NewRequest(
				http.MethodGet,
				basePath+tt.query,
				nil,
			)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
			}

			if service.syncCalls != 1 ||
				service.gotSyncChatID != chatID ||
				service.gotSyncUserID != userID ||
				service.gotSyncAfterSeq != tt.wantAfter ||
				service.gotSyncLimit != tt.wantLimit {
				t.Fatal("unexpected service call arguments")
			}

			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatal("expected JSON content type")
			}

			var response MessageSyncResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.Items == nil {
				t.Fatal("items must be an array, not null")
			}

			if len(response.Items) != len(page.Items) {
				t.Fatal("unexpected number of response items")
			}

			for i, got := range response.Items {
				want := page.Items[i]

				if got.ID != want.ID ||
					got.ChatID != want.ChatID ||
					got.SenderID != want.SenderID ||
					got.ClientMessageID != want.ClientMessageID ||
					got.Text != want.Text ||
					got.Seq != want.Seq ||
					!got.CreatedAt.Equal(want.CreatedAt) {
					t.Fatalf("unexpected response item %d: %+v", i, got)
				}
			}

			if page.NextAfterSeq == nil {
				if response.NextAfterSeq != nil {
					t.Fatal("expected null continuation cursor")
				}
			} else if response.NextAfterSeq == nil ||
				*response.NextAfterSeq != *page.NextAfterSeq {
				t.Fatal("unexpected continuation cursor")
			}
		})
	}
}

func TestMessageHandler_SyncChatMessagesRejectRequest(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	basePath := "/chats/" + chatID.String() + "/messages/sync"

	tests := []struct {
		name        string
		path        string
		withoutAuth bool
		wantStatus  int
	}{
		{"no authentication", basePath + "?after_seq=0", true, 401},
		{"invalid chat UUID", "/chats/invalid/messages/sync?after_seq=0", false, 400},
		{"nil chat UUID", "/chats/" + uuid.Nil.String() + "/messages/sync?after_seq=0", false, 400},
		{"missing cursor", basePath, false, 400},
		{"empty cursor", basePath + "?after_seq=", false, 400},
		{"negative cursor", basePath + "?after_seq=-1", false, 400},
		{"non-integer cursor", basePath + "?after_seq=abc", false, 400},
		{"overflow cursor", basePath + "?after_seq=9223372036854775808", false, 400},
		{"duplicate cursor", basePath + "?after_seq=0&after_seq=1", false, 400},
		{"empty limit", basePath + "?after_seq=0&limit=", false, 400},
		{"zero limit", basePath + "?after_seq=0&limit=0", false, 400},
		{"oversized limit", basePath + "?after_seq=0&limit=101", false, 400},
		{"duplicate limit", basePath + "?after_seq=0&limit=1&limit=2", false, 400},
		{"invalid query separator", basePath + "?after_seq=0;limit=1", false, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockMessageService{
				syncPage: &domain.MessageSyncPage{},
			}
			router, token := newMessageTestRouter(t, service, userID)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
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
					rec.Body,
				)
			}

			if service.syncCalls != 0 {
				t.Fatal("rejected request must not reach service")
			}
		})
	}
}

func TestMessageHandler_SyncChatMessagesServiceErrors(t *testing.T) {
	userID := uuid.New()
	chatID := uuid.New()

	const privateDetail = "private-database-password"

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{
			name: "invalid input",
			serviceErr: fmt.Errorf(
				"sync: %w",
				domain.ErrInvalidInput,
			),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "not a member",
			serviceErr: fmt.Errorf(
				"sync: %w",
				domain.ErrNotChatMember,
			),
			wantStatus: http.StatusForbidden,
		},
		{
			name: "missing chat",
			serviceErr: fmt.Errorf(
				"sync: %w",
				domain.ErrChatNotFound,
			),
			wantStatus: http.StatusNotFound,
		},
		{
			name: "unexpected failure",
			serviceErr: fmt.Errorf(
				"database connection failed: %s",
				privateDetail,
			),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &mockMessageService{
				syncErr: tt.serviceErr,
			}
			router, token := newMessageTestRouter(t, service, userID)

			path := "/chats/" + chatID.String() +
				"/messages/sync?after_seq=7&limit=2"

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"expected %d, got %d: %s",
					tt.wantStatus,
					rec.Code,
					rec.Body,
				)
			}

			if service.syncCalls != 1 {
				t.Fatalf(
					"expected one service call, got %d",
					service.syncCalls,
				)
			}

			if strings.Contains(rec.Body.String(), privateDetail) {
				t.Fatal("response exposed internal error details")
			}
		})
	}
}

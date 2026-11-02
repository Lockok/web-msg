package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// Сохраняет совместимость существующих тестов с расширенным интерфейсом.
func (m *mockMessageService) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	limit int,
	beforeSeq *int64,
) (*domain.MessagePage, error) {
	return nil, errors.New("unexpected search call")
}

type searchServiceStub struct {
	*mockMessageService
	page       *domain.MessagePage
	err        error
	calls      int
	chatID     uuid.UUID
	userID     uuid.UUID
	searchText string
	limit      int
	beforeSeq  *int64
}

func (m *searchServiceStub) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	limit int,
	beforeSeq *int64,
) (*domain.MessagePage, error) {
	m.calls++
	m.chatID = chatID
	m.userID = userID
	m.searchText = searchText
	m.limit = limit
	m.beforeSeq = beforeSeq
	return m.page, m.err
}

func TestMessageHandler_SearchChatMessages(t *testing.T) {
	userID := uuid.New()
	chatID := uuid.New()
	next := int64(4)

	svc := &searchServiceStub{
		page: &domain.MessagePage{
			Items:         []domain.Message{{Seq: 5}, {Seq: 4}},
			NextBeforeSeq: &next,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewMessageHandler(svc, logger)
	jwtManager := auth.NewJWTManager("test-secret")

	token, err := jwtManager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := chi.NewRouter()
	router.With(middleware.Auth(jwtManager)).Get(
		"/chats/{chat_id}/messages/search",
		handler.SearchChatMessages,
	)

	base := "/chats/" + chatID.String() + "/messages/search"
	req := httptest.NewRequest(
		http.MethodGet, base+"?q=hello&limit=2&before_seq=7", nil,
	)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	if svc.calls != 1 || svc.chatID != chatID || svc.userID != userID ||
		svc.searchText != "hello" || svc.limit != 2 ||
		svc.beforeSeq == nil || *svc.beforeSeq != 7 {
		t.Fatalf("unexpected service call: %+v", svc)
	}

	var response MessageHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 2 || response.NextBeforeSeq == nil ||
		*response.NextBeforeSeq != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}

	// Некорректные параметры не должны доходить до сервиса.
	for _, query := range []string{
		"", "?q=a&q=b", "?q=a&limit=0",
		"?q=a&limit=1&limit=2", "?q=a&before_seq=0",
	} {
		svc.calls = 0

		req := httptest.NewRequest(http.MethodGet, base+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest || svc.calls != 0 {
			t.Fatalf("query %q: status=%d calls=%d", query, rec.Code, svc.calls)
		}
	}

	svc.calls = 0
	req = httptest.NewRequest(http.MethodGet, base+"?q=hello", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || svc.calls != 0 {
		t.Fatalf("unauthorized: status=%d calls=%d", rec.Code, svc.calls)
	}

	svc.err = domain.ErrNotChatMember
	req = httptest.NewRequest(http.MethodGet, base+"?q=hello", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

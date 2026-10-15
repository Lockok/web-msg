package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/service"
)

func TestChatRoutesWithoutAuthorization(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtManager := auth.NewJWTManager("test-secret")

	// Repository не нужны: запросы без авторизации
	// должны завершаться до обращения к бизнес-логике.
	userService := service.NewUserService(nil)
	chatService := service.NewChatService(nil)
	messageService := service.NewMessageService(nil)

	router := New(*userService, chatService, messageService, jwtManager, log)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "chat list",
			method: http.MethodGet,
			path:   "/chats",
		},
		{
			name:   "private chat creation",
			method: http.MethodPost,
			path:   "/chats/private",
		},
		{
			name:   "message sending",
			method: http.MethodPost,
			path:   "/chats/00000000-0000-0000-0000-000000000010/messages",
		},

		{
			name:   "message history",
			method: http.MethodGet,
			path:   "/chats/00000000-0000-0000-0000-000000000010/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf(
					"%s %s: expected 401, got %d: %s",
					tt.method,
					tt.path,
					rec.Code,
					rec.Body.String(),
				)
			}
		})
	}
}

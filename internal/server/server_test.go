package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/realtime"
	"github.com/Lockok/web-msg/internal/service"
)

func TestChatRoutesWithoutAuthorization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtManager := auth.NewJWTManager("test-secret")

	// Repository не нужны: запросы без авторизации
	// должны завершаться до обращения к бизнес-логике.
	userService := service.NewUserService(nil)
	chatService := service.NewChatService(nil)
	messageService := service.NewMessageService(nil)

	router := New(*userService, chatService, messageService, jwtManager, logger, realtime.NewHub())

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
		{
			name:   "message sync",
			method: http.MethodGet,
			path:   "/chats/00000000-0000-0000-0000-000000000010/messages/sync?after_seq=0",
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

func TestWebSocketRoute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtManager := auth.NewJWTManager("websocket-route-test-secret")

	userService := service.NewUserService(nil)
	chatService := service.NewChatService(nil)
	messageService := service.NewMessageService(nil)

	router := New(
		*userService,
		chatService,
		messageService,
		jwtManager,
		logger,
		realtime.NewHub(),
	)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := jwtManager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect through /ws route: %v", err)
	}
	defer conn.CloseNow()

	err = wsjson.Write(ctx, conn, map[string]string{
		"type":         "auth",
		"access_token": token,
	})
	if err != nil {
		t.Fatalf("send auth: %v", err)
	}

	var response struct {
		Type string `json:"type"`
	}

	if err := wsjson.Read(ctx, conn, &response); err != nil {
		t.Fatalf("read auth response: %v", err)
	}

	if response.Type != "auth.ok" {
		t.Fatalf("expected auth.ok, got %q", response.Type)
	}

	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close connection: %v", err)
	}
}

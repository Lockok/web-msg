package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/realtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newWebSocketTestServer(
	t *testing.T,
) (string, *auth.JWTManager) {
	t.Helper()

	jwtManager := auth.NewJWTManager("websocket-test-secret")
	hub := realtime.NewHub()
	handler := NewWebSocketHandler(jwtManager, hub)

	server := httptest.NewServer(
		http.HandlerFunc(handler.Connect),
	)
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	return url, jwtManager
}

func TestWebSocketHandler_Authenticate(t *testing.T) {
	url, jwtManager := newWebSocketTestServer(t)

	token, err := jwtManager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.CloseNow()

	err = wsjson.Write(ctx, conn, wsAuthRequest{
		Type:        "auth",
		AccessToken: token,
	})
	if err != nil {
		t.Fatalf("send auth: %v", err)
	}

	var response wsAuthResponse
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

func TestWebSocketHandler_RejectAuthentication(t *testing.T) {
	url, jwtManager := newWebSocketTestServer(t)

	token, err := jwtManager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	tests := []struct {
		name       string
		payload    string
		wantStatus websocket.StatusCode
	}{
		{
			name:       "invalid token",
			payload:    `{"type":"auth","access_token":"invalid"}`,
			wantStatus: websocket.StatusPolicyViolation,
		},
		{
			name:       "missing token",
			payload:    `{"type":"auth"}`,
			wantStatus: websocket.StatusPolicyViolation,
		},
		{
			name: "wrong first message type",
			payload: `{"type":"subscribe","access_token":"` +
				token + `"}`,
			wantStatus: websocket.StatusPolicyViolation,
		},
		{
			name:       "malformed JSON",
			payload:    `{"type":`,
			wantStatus: websocket.StatusInvalidFramePayloadData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			conn, _, err := websocket.Dial(ctx, url, nil)
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			defer conn.CloseNow()

			err = conn.Write(
				ctx,
				websocket.MessageText,
				[]byte(tt.payload),
			)
			if err != nil {
				t.Fatalf("send first message: %v", err)
			}

			_, _, err = conn.Read(ctx)
			if err == nil {
				t.Fatal("expected connection to be closed")
			}

			if status := websocket.CloseStatus(err); status != tt.wantStatus {
				t.Fatalf(
					"expected close status %d, got %d: %v",
					tt.wantStatus,
					status,
					err,
				)
			}
		})
	}
}

func TestWebSocketHandler_AuthenticationTimeout(t *testing.T) {
	url, _ := newWebSocketTestServer(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		wsAuthTimeout+5*time.Second,
	)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.CloseNow()

	// Ничего не отправляем: сервер должен перестать ждать JWT.
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("expected unauthenticated connection to close")
	}

	// При истечении контекста чтения библиотека может закрыть
	// транспорт без отправки WebSocket close-frame.
	// Поэтому здесь не требуем конкретного close status.
	if ctx.Err() != nil {
		t.Fatalf(
			"server did not close connection before client timeout: %v",
			err,
		)
	}
}

func TestWebSocketHandler_CloseWhenTokenExpires(t *testing.T) {
	url, _ := newWebSocketTestServer(t)

	expiresAt := time.Now().
		Add(3 * time.Second).
		Truncate(time.Second)

	claims := jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	token, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte("websocket-test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.CloseNow()

	err = wsjson.Write(ctx, conn, wsAuthRequest{
		Type:        "auth",
		AccessToken: token,
	})
	if err != nil {
		t.Fatalf("send auth: %v", err)
	}

	var response wsAuthResponse
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		t.Fatalf("read auth response: %v", err)
	}

	if response.Type != "auth.ok" {
		t.Fatalf("expected auth.ok, got %q", response.Type)
	}

	// Теперь ждём закрытия сервером, ничего не отправляя.
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("expected connection to close after token expiration")
	}

	if ctx.Err() != nil {
		t.Fatalf("server did not close connection in time: %v", err)
	}

	if time.Now().Before(expiresAt) {
		t.Fatal("connection closed before token expiration")
	}

	if status := websocket.CloseStatus(err); status !=
		websocket.StatusPolicyViolation {
		t.Fatalf(
			"expected policy violation, got status %d: %v",
			status,
			err,
		)
	}
}

func TestWebSocketHandler_RegistersAndRemovesConnection(t *testing.T) {
	jwtManager := auth.NewJWTManager("websocket-lifecycle-test-secret")
	hub := realtime.NewHub()

	handler := NewWebSocketHandler(jwtManager, hub)

	server := httptest.NewServer(
		http.HandlerFunc(handler.Connect),
	)
	t.Cleanup(server.Close)

	userID := uuid.New()

	token, err := jwtManager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.CloseNow()

	if got := len(hub.Connections(userID)); got != 0 {
		t.Fatalf(
			"expected no connections before authentication, got %d",
			got,
		)
	}

	err = wsjson.Write(ctx, conn, wsAuthRequest{
		Type:        "auth",
		AccessToken: token,
	})
	if err != nil {
		t.Fatalf("send auth: %v", err)
	}

	var response wsAuthResponse
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		t.Fatalf("read auth response: %v", err)
	}

	if response.Type != "auth.ok" {
		t.Fatalf("expected auth.ok, got %q", response.Type)
	}

	if got := len(hub.Connections(userID)); got != 1 {
		t.Fatalf(
			"expected one authenticated connection, got %d",
			got,
		)
	}

	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close connection: %v", err)
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(
		ctx,
		2*time.Second,
	)
	defer cancelCleanup()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(hub.Connections(userID)) == 0 {
			break
		}

		select {
		case <-ticker.C:
		case <-cleanupCtx.Done():
			t.Fatal("disconnected client remained in Hub")
		}
	}
}

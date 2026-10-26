//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/realtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

func TestMessageRealtime(t *testing.T) {
	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	post := func(
		path string,
		token string,
		body any,
		result any,
		wantStatus int,
	) {
		t.Helper()

		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			baseURL+path,
			bytes.NewReader(data),
		)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer response.Body.Close()

		if response.StatusCode != wantStatus {
			t.Fatalf(
				"POST %s: expected %d, got %d",
				path,
				wantStatus,
				response.StatusCode,
			)
		}

		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			t.Fatalf("decode POST %s response: %v", path, err)
		}
	}

	createAccount := func() (uuid.UUID, string) {
		t.Helper()

		credentials := map[string]string{
			"username": "e2e_" +
				strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
			"password": "LocalTest_12345!",
		}

		var user struct {
			ID uuid.UUID `json:"id"`
		}
		post("/register", "", credentials, &user, http.StatusCreated)

		var login struct {
			AccessToken string `json:"access_token"`
		}
		post("/login", "", credentials, &login, http.StatusOK)

		if user.ID == uuid.Nil || login.AccessToken == "" {
			t.Fatal("registration or login returned incomplete data")
		}

		return user.ID, login.AccessToken
	}

	senderID, senderToken := createAccount()
	receiverID, receiverToken := createAccount()

	var chat struct {
		ID uuid.UUID `json:"id"`
	}
	post(
		"/chats/private",
		senderToken,
		map[string]string{"peer_id": receiverID.String()},
		&chat,
		http.StatusOK,
	)

	if chat.ID == uuid.Nil {
		t.Fatal("chat response contains no ID")
	}

	wsURL := strings.Replace(baseURL, "http", "ws", 1) + "/ws"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("connect receiver WebSocket: %v", err)
	}
	defer conn.CloseNow()

	err = wsjson.Write(ctx, conn, map[string]string{
		"type":         "auth",
		"access_token": receiverToken,
	})
	if err != nil {
		t.Fatalf("authenticate receiver: %v", err)
	}

	var authResponse struct {
		Type string `json:"type"`
	}
	if err := wsjson.Read(ctx, conn, &authResponse); err != nil {
		t.Fatalf("read authentication response: %v", err)
	}

	if authResponse.Type != "auth.ok" {
		t.Fatalf("expected auth.ok, got %q", authResponse.Type)
	}

	clientMessageID := uuid.New()
	text := "  Привет 👋\nПроверяем realtime  "

	var saved struct {
		ID uuid.UUID `json:"id"`
	}

	post(
		"/chats/"+chat.ID.String()+"/messages",
		senderToken,
		map[string]string{
			"client_message_id": clientMessageID.String(),
			"text":              text,
		},
		&saved,
		http.StatusOK,
	)

	if saved.ID == uuid.Nil {
		t.Fatal("HTTP response contains no message ID")
	}

	var event realtime.MessageCreatedEvent
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("receiver did not receive message event: %v", err)
	}

	if event.Type != "message.created" ||
		event.Message.ID != saved.ID ||
		event.Message.ChatID != chat.ID ||
		event.Message.SenderID != senderID ||
		event.Message.ClientMessageID != clientMessageID ||
		event.Message.Text != text ||
		event.Message.Seq != 1 {
		t.Fatalf("unexpected realtime event: %+v", event)
	}
}

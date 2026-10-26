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
	_, outsiderToken := createAccount()

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

	connect := func(token string) *websocket.Conn {
		t.Helper()

		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("connect WebSocket: %v", err)
		}

		t.Cleanup(func() {
			_ = conn.CloseNow()
		})

		err = wsjson.Write(ctx, conn, map[string]string{
			"type":         "auth",
			"access_token": token,
		})
		if err != nil {
			t.Fatalf("authenticate WebSocket: %v", err)
		}

		var response struct {
			Type string `json:"type"`
		}
		if err := wsjson.Read(ctx, conn, &response); err != nil {
			t.Fatalf("read authentication response: %v", err)
		}

		if response.Type != "auth.ok" {
			t.Fatalf("expected auth.ok, got %q", response.Type)
		}

		return conn
	}

	receiverTab1 := connect(receiverToken)
	receiverTab2 := connect(receiverToken)
	outsiderConn := connect(outsiderToken)

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

	expectEvent := func(
		conn *websocket.Conn,
		messageID uuid.UUID,
		clientID uuid.UUID,
		wantText string,
		wantSeq int64,
	) {
		t.Helper()

		var event realtime.MessageCreatedEvent
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatalf("read message event: %v", err)
		}

		if event.Type != "message.created" ||
			event.Message.ID != messageID ||
			event.Message.ChatID != chat.ID ||
			event.Message.SenderID != senderID ||
			event.Message.ClientMessageID != clientID ||
			event.Message.Text != wantText ||
			event.Message.Seq != wantSeq {
			t.Fatalf("unexpected realtime event: %+v", event)
		}
	}

	// Каждая вкладка должна получить первое сообщение.
	expectEvent(receiverTab1, saved.ID, clientMessageID, text, 1)
	expectEvent(receiverTab2, saved.ID, clientMessageID, text, 1)

	// Повторяем тот же запрос с тем же ключом и текстом.
	var repeated struct {
		ID uuid.UUID `json:"id"`
	}

	post(
		"/chats/"+chat.ID.String()+"/messages",
		senderToken,
		map[string]string{
			"client_message_id": clientMessageID.String(),
			"text":              text,
		},
		&repeated,
		http.StatusOK,
	)

	if repeated.ID != saved.ID {
		t.Fatal("repeated request returned a different message")
	}

	// Отправляем новое сообщение-маркер.
	secondClientID := uuid.New()
	secondText := "Второе сообщение"

	var second struct {
		ID uuid.UUID `json:"id"`
	}

	post(
		"/chats/"+chat.ID.String()+"/messages",
		senderToken,
		map[string]string{
			"client_message_id": secondClientID.String(),
			"text":              secondText,
		},
		&second,
		http.StatusOK,
	)

	if second.ID == uuid.Nil || second.ID == saved.ID {
		t.Fatal("expected a new message ID")
	}

	// Следующим событием должно быть второе сообщение, не дубль первого.
	// seq=2 также проверяет, что повтор не увеличил счётчик.
	expectEvent(receiverTab1, second.ID, secondClientID, secondText, 2)
	expectEvent(receiverTab2, second.ID, secondClientID, secondText, 2)

	// Посторонний не должен получить ни одно из событий.
	outsiderCtx, cancelOutsider := context.WithTimeout(
		ctx,
		300*time.Millisecond,
	)
	defer cancelOutsider()

	_, _, err := outsiderConn.Read(outsiderCtx)
	if err == nil {
		t.Fatal("outsider received an unexpected message")
	}

	if outsiderCtx.Err() != context.DeadlineExceeded {
		t.Fatalf("outsider connection failed unexpectedly: %v", err)
	}
}

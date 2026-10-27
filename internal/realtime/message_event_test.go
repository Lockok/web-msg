package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

func TestWriteMessageCreated(t *testing.T) {
	message := domain.Message{
		ID:              uuid.New(),
		ChatID:          uuid.New(),
		SenderID:        uuid.New(),
		ClientMessageID: uuid.New(),
		Text:            "  Привет 👋\nВторая строка  ",
		Seq:             7,
		CreatedAt:       time.Now().UTC(),
	}

	writeResult := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				writeResult <- err
				return
			}
			defer conn.CloseNow()

			err = WriteMessageCreated(r.Context(), conn, message)
			writeResult <- err

			if err == nil {
				_ = conn.Close(websocket.StatusNormalClosure, "")
			}
		},
	))
	t.Cleanup(server.Close)

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

	var event MessageCreatedEvent
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("read event: %v", err)
	}

	if event.Type != "message.created" {
		t.Fatalf("unexpected event type: %q", event.Type)
	}

	got := event.Message

	if got.ID != message.ID ||
		got.ChatID != message.ChatID ||
		got.SenderID != message.SenderID ||
		got.ClientMessageID != message.ClientMessageID ||
		got.Text != message.Text ||
		got.Seq != message.Seq ||
		!got.CreatedAt.Equal(message.CreatedAt) {
		t.Fatalf("unexpected message payload: %+v", got)
	}

	select {
	case err := <-writeResult:
		if err != nil {
			t.Fatalf("write event: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("server did not report write result")
	}

	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close connection: %v", err)
	}
}

func TestWriteMessageCreated_ClosedConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	start := make(chan struct{})
	writeResult := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				writeResult <- err
				return
			}
			defer conn.CloseNow()

			select {
			case <-start:
			case <-ctx.Done():
				return
			}

			_ = conn.CloseNow()

			writeResult <- WriteMessageCreated(
				ctx,
				conn,
				domain.Message{Text: "hello"},
			)
		},
	))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.CloseNow()

	// Handshake завершён. Разрешаем серверу закрыть соединение
	// и попытаться записать событие.
	close(start)

	select {
	case err := <-writeResult:
		if err == nil {
			t.Fatal("expected error when writing to closed connection")
		}
	case <-ctx.Done():
		t.Fatal("server did not report write result")
	}
}

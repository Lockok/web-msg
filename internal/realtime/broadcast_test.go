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

func newBroadcastTestConnection(
	t *testing.T,
) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	accepted := make(chan *websocket.Conn, 1)
	acceptErrors := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				acceptErrors <- err
				return
			}
			defer conn.CloseNow()

			accepted <- conn

			ctx := conn.CloseRead(r.Context())
			<-ctx.Done()
		},
	))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	clientConn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = clientConn.CloseNow()
	})

	select {
	case serverConn := <-accepted:
		t.Cleanup(func() {
			_ = serverConn.CloseNow()
		})
		return serverConn, clientConn

	case err := <-acceptErrors:
		t.Fatalf("accept connection: %v", err)

	case <-ctx.Done():
		t.Fatal("server did not accept connection in time")
	}

	return nil, nil
}

func TestHub_BroadcastMessageCreatedRecipients(t *testing.T) {
	hub := NewHub()

	recipientID := uuid.New()
	outsiderID := uuid.New()

	serverTab1, clientTab1 := newBroadcastTestConnection(t)
	serverTab2, clientTab2 := newBroadcastTestConnection(t)
	serverOutsider, clientOutsider := newBroadcastTestConnection(t)

	for _, registration := range []struct {
		userID uuid.UUID
		conn   *websocket.Conn
	}{
		{recipientID, serverTab1},
		{recipientID, serverTab2},
		{outsiderID, serverOutsider},
	} {
		if err := hub.Register(
			registration.userID,
			registration.conn,
		); err != nil {
			t.Fatalf("register: %v", err)
		}
	}

	message := domain.Message{
		ID:   uuid.New(),
		Text: "Привет 👋",
		Seq:  3,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	report := hub.BroadcastMessageCreated(
		ctx,
		[]uuid.UUID{recipientID, recipientID},
		message,
	)

	want := BroadcastReport{
		Connections: 2,
		Written:     2,
		Failed:      0,
	}
	if report != want {
		t.Fatalf("expected report %+v, got %+v", want, report)
	}

	for i, conn := range []*websocket.Conn{clientTab1, clientTab2} {
		var event MessageCreatedEvent
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatalf("read event from tab %d: %v", i+1, err)
		}

		if event.Type != "message.created" ||
			event.Message.ID != message.ID ||
			event.Message.Text != message.Text ||
			event.Message.Seq != message.Seq {
			t.Fatalf("unexpected event in tab %d: %+v", i+1, event)
		}
	}

	outsiderCtx, cancelOutsider := context.WithTimeout(
		ctx,
		200*time.Millisecond,
	)
	defer cancelOutsider()

	_, _, err := clientOutsider.Read(outsiderCtx)
	if err == nil {
		t.Fatal("outsider received an unexpected message")
	}

	if outsiderCtx.Err() != context.DeadlineExceeded {
		t.Fatalf("outsider connection failed unexpectedly: %v", err)
	}
}

func TestHub_BroadcastMessageCreatedPartialFailure(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	liveServer, liveClient := newBroadcastTestConnection(t)
	closedServer, _ := newBroadcastTestConnection(t)

	for _, conn := range []*websocket.Conn{liveServer, closedServer} {
		if err := hub.Register(userID, conn); err != nil {
			t.Fatalf("register: %v", err)
		}
	}

	_ = closedServer.CloseNow()

	message := domain.Message{
		ID:   uuid.New(),
		Text: "hello",
		Seq:  1,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	report := hub.BroadcastMessageCreated(
		ctx,
		[]uuid.UUID{userID},
		message,
	)

	want := BroadcastReport{
		Connections: 2,
		Written:     1,
		Failed:      1,
	}
	if report != want {
		t.Fatalf("expected report %+v, got %+v", want, report)
	}

	var event MessageCreatedEvent
	if err := wsjson.Read(ctx, liveClient, &event); err != nil {
		t.Fatalf("read event from live connection: %v", err)
	}

	if event.Type != "message.created" ||
		event.Message.ID != message.ID {
		t.Fatalf("unexpected event: %+v", event)
	}
}

package realtime

import (
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

func TestHub_ConnectionLifecycle(t *testing.T) {
	hub := NewHub()

	aliceID := uuid.New()
	bobID := uuid.New()

	aliceTab1 := &websocket.Conn{}
	aliceTab2 := &websocket.Conn{}
	bobTab := &websocket.Conn{}

	for _, registration := range []struct {
		userID uuid.UUID
		conn   *websocket.Conn
	}{
		{aliceID, aliceTab1},
		{aliceID, aliceTab2},
		{bobID, bobTab},
		{aliceID, aliceTab1}, // Повторная регистрация.
	} {
		if err := hub.Register(
			registration.userID,
			registration.conn,
		); err != nil {
			t.Fatalf("register connection: %v", err)
		}
	}

	aliceConnections := hub.Connections(aliceID)
	if len(aliceConnections) != 2 {
		t.Fatalf(
			"expected 2 Alice connections, got %d",
			len(aliceConnections),
		)
	}

	found := make(map[*websocket.Conn]bool)
	for _, conn := range aliceConnections {
		found[conn] = true
	}

	if !found[aliceTab1] || !found[aliceTab2] {
		t.Fatal("Alice connections do not match her tabs")
	}

	bobConnections := hub.Connections(bobID)
	if len(bobConnections) != 1 || bobConnections[0] != bobTab {
		t.Fatal("Bob must have only his own connection")
	}

	hub.Unregister(aliceID, aliceTab1)

	remaining := hub.Connections(aliceID)
	if len(remaining) != 1 || remaining[0] != aliceTab2 {
		t.Fatal("removing one tab must preserve the other tab")
	}

	hub.Unregister(aliceID, aliceTab2)

	if got := len(hub.Connections(aliceID)); got != 0 {
		t.Fatalf("expected no Alice connections, got %d", got)
	}

	// Повторное удаление должно быть безопасным.
	hub.Unregister(aliceID, aliceTab2)
	hub.Unregister(uuid.New(), aliceTab1)

	bobConnections = hub.Connections(bobID)
	if len(bobConnections) != 1 || bobConnections[0] != bobTab {
		t.Fatal("removing Alice connections must not affect Bob")
	}
}

func TestHub_RegisterInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		userID uuid.UUID
		conn   *websocket.Conn
	}{
		{
			name:   "empty user ID",
			userID: uuid.Nil,
			conn:   &websocket.Conn{},
		},
		{
			name:   "nil connection",
			userID: uuid.New(),
			conn:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub()

			if err := hub.Register(tt.userID, tt.conn); err == nil {
				t.Fatal("expected registration error")
			}

			if got := len(hub.Connections(tt.userID)); got != 0 {
				t.Fatalf(
					"invalid registration added %d connections",
					got,
				)
			}
		})
	}
}

func TestHub_ConcurrentLifecycle(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	const (
		workers    = 16
		iterations = 50
	)

	start := make(chan struct{})
	failures := make(chan error, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// У каждой горутины своё соединение.
			conn := &websocket.Conn{}

			// Ждём общего сигнала старта.
			<-start

			for j := 0; j < iterations; j++ {
				if err := hub.Register(userID, conn); err != nil {
					failures <- err
					return
				}

				connections := hub.Connections(userID)

				found := false
				for _, candidate := range connections {
					if candidate == conn {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("registered connection is missing")
				}

				hub.Unregister(userID, conn)
			}
		}()
	}

	close(start)
	wg.Wait()
	close(failures)

	for err := range failures {
		t.Errorf("register connection: %v", err)
	}

	if got := len(hub.Connections(userID)); got != 0 {
		t.Fatalf("expected empty registry, got %d connections", got)
	}
}

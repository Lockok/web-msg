package realtime

import (
	"errors"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Hub struct {
	mu sync.RWMutex

	connections map[uuid.UUID]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[uuid.UUID]map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) Register(userID uuid.UUID, conn *websocket.Conn) error {
	if userID == uuid.Nil {
		return errors.New("connection user ID must be not empty")
	}

	if conn == nil {
		return errors.New("connection must not be nill")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connections[userID] == nil {
		h.connections[userID] = make(map[*websocket.Conn]struct{})
	}

	h.connections[userID][conn] = struct{}{}

	return nil
}

func (h *Hub) Unregister(userID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	connections := h.connections[userID]
	delete(connections, conn)

	if len(connections) == 0 {
		delete(h.connections, userID)
	}
}

func (h *Hub) Connections(userID uuid.UUID) []*websocket.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()

	connections := h.connections[userID]

	result := make([]*websocket.Conn, 0, len(connections))
	for conn := range connections {
		result = append(result, conn)
	}

	return result
}

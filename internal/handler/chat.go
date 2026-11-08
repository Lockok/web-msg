package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

type ChatService interface {
	GetOrCreatePrivateChat(ctx context.Context, userID uuid.UUID, peerID uuid.UUID) (*domain.Chat, error)
	ListUserChats(ctx context.Context, userID uuid.UUID, limit int, cursor *domain.ChatCursor) (*domain.ChatPage, error)
	CreateGroupChat(ctx context.Context, input domain.CreateGroupChatInput) (*domain.Chat, error)
	SetGroupMember(ctx context.Context, chatID, actorID, targetID uuid.UUID, add bool) error
	ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) (int64, error)
}

type ChatHandler struct {
	chats ChatService
	log   *slog.Logger
}

func NewChatHandler(chats ChatService, log *slog.Logger) *ChatHandler {
	return &ChatHandler{
		chats: chats,
		log:   log,
	}
}

type PrivateChatRequest struct {
	PeerID string `json:"peer_id"`
}

type ChatResponse struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *ChatHandler) GetOrCreatePrivateChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req PrivateChatRequest

	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	peerID, err := uuid.Parse(req.PeerID)
	if err != nil || peerID == uuid.Nil {
		http.Error(w, "peer_id must be a non-empty UUID", http.StatusBadRequest)
		return
	}

	chat, err := h.chats.GetOrCreatePrivateChat(r.Context(), userID, peerID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid chat request", http.StatusBadRequest)

		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)

		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to get or create private chat",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)

			http.Error(w, "internal server error", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(ChatResponse{
		ID:        chat.ID,
		Type:      chat.Type,
		Name:      chat.Name,
		CreatedAt: chat.CreatedAt,
	})
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

type CreateGroupChatRequest struct {
	Name string `json:"name"`
}

func (h *ChatHandler) CreateGroupChat(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateGroupChatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	chat, err := h.chats.CreateGroupChat(
		r.Context(),
		domain.CreateGroupChatInput{
			CreatorID: userID,
			Name:      req.Name,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid group name", http.StatusBadRequest)

		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)

		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to create group chat",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(ChatResponse{
		ID:        chat.ID,
		Type:      chat.Type,
		Name:      chat.Name,
		CreatedAt: chat.CreatedAt,
	})
}

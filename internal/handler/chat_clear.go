package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

func (h *ChatHandler) ClearChatHistory(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil || chatID == uuid.Nil {
		http.Error(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	clearedThrough, err := h.chats.ClearChatHistory(r.Context(), chatID, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid clearance request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotChatMember):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrChatNotFound):
			http.Error(w, "chat not found", http.StatusNotFound)
		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to clear chat history",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		ClearedThroughSeq int64 `json:"cleared_through_seq"`
	}{
		ClearedThroughSeq: clearedThrough,
	})
}

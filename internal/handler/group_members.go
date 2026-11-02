package handler

import (
	"errors"
	"net/http"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

func (h *ChatHandler) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	h.setGroupMember(w, r, true)
}

func (h *ChatHandler) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	h.setGroupMember(w, r, false)
}

func (h *ChatHandler) setGroupMember(
	w http.ResponseWriter,
	r *http.Request,
	add bool,
) {
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok || actorID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil || chatID == uuid.Nil {
		http.Error(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil || targetID == uuid.Nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	err = h.chats.SetGroupMember(r.Context(), chatID, actorID, targetID, add)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid membership request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotChatMember):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrChatNotFound):
			http.Error(w, "chat not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)
		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to change group membership",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

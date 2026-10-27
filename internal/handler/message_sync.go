package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type MessageSyncResponse struct {
	Items        []MessageResponse `json:"items"`
	NextAfterSeq *int64            `json:"next_after_seq"`
}

func (h *MessageHandler) SyncChatMessages(
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
		http.Error(w, "chat_id must be a non-empty UUID", http.StatusBadRequest)
		return
	}

	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid query parameters", http.StatusBadRequest)
		return
	}

	if len(params["after_seq"]) != 1 || len(params["limit"]) > 1 {
		http.Error(
			w,
			"after_seq is required; duplicate parameters are not allowed",
			http.StatusBadRequest,
		)
		return
	}

	afterSeq, err := strconv.ParseInt(params["after_seq"][0], 10, 64)
	if err != nil || afterSeq < 0 {
		http.Error(
			w,
			"after_seq must be a non-negative int64",
			http.StatusBadRequest,
		)
		return
	}

	limit := 100

	if values, exists := params["limit"]; exists {
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
			return
		}
	}

	page, err := h.messages.SyncChatMessages(
		r.Context(),
		chatID,
		userID,
		afterSeq,
		limit,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid sync request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotChatMember):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrChatNotFound):
			http.Error(w, "chat not found", http.StatusNotFound)
		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to sync chat messages",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}

		return
	}

	response := MessageSyncResponse{
		Items:        make([]MessageResponse, 0, len(page.Items)),
		NextAfterSeq: page.NextAfterSeq,
	}

	for _, message := range page.Items {
		response.Items = append(response.Items, MessageResponse{
			ID:              message.ID,
			ChatID:          message.ChatID,
			SenderID:        message.SenderID,
			ClientMessageID: message.ClientMessageID,
			Text:            message.Text,
			Seq:             message.Seq,
			CreatedAt:       message.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

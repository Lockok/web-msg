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

type MessageHistoryResponse struct {
	Items         []MessageResponse `json:"items"`
	NextBeforeSeq *int64            `json:"next_before_seq"`
}

func (h *MessageHandler) ListChatMessages(
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

	if len(params["limit"]) > 1 || len(params["before_seq"]) > 1 {
		http.Error(w, "duplicate query parameters", http.StatusBadRequest)
		return
	}

	limit := 20

	if values, exists := params["limit"]; exists {
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
			return
		}
	}

	var beforeSeq *int64

	if values, exists := params["before_seq"]; exists {
		value, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil || value <= 0 {
			http.Error(w, "before_seq must be a positive int64", http.StatusBadRequest)
			return
		}

		beforeSeq = &value
	}

	page, err := h.messages.ListChatMessages(
		r.Context(),
		chatID,
		userID,
		limit,
		beforeSeq,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid history request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotChatMember):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrChatNotFound):
			http.Error(w, "chat not found", http.StatusNotFound)
		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to get message history",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)

			http.Error(w, "internal server error", http.StatusInternalServerError)
		}

		return
	}

	response := MessageHistoryResponse{
		Items:         make([]MessageResponse, 0, len(page.Items)),
		NextBeforeSeq: page.NextBeforeSeq,
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

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
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

const maxMessageBodyBytes int64 = 32 * 1024

type MessageService interface {
	Send(ctx context.Context, input domain.SendMessageInput) (*domain.Message, error)
}

type MessageHandler struct {
	messages MessageService
	log      *slog.Logger
}

func NewMessageHandler(messages MessageService, log *slog.Logger) *MessageHandler {
	return &MessageHandler{
		messages: messages,
		log:      log,
	}
}

type SendMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Text            string `json:"text"`
}

type MessageResponse struct {
	ID              uuid.UUID `json:"id"`
	ChatID          uuid.UUID `json:"chat_id"`
	SenderID        uuid.UUID `json:"sender_id"`
	ClientMessageID uuid.UUID `json:"client_message_id"`
	Text            string    `json:"text"`
	Seq             int64     `json:"seq"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	senderID, ok := middleware.GetUserID(r.Context())
	if !ok || senderID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil || chatID == uuid.Nil {
		http.Error(w, "chat_id must be a non-empty UUID", http.StatusBadRequest)
		return
	}

	var req SendMessageRequest

	if err := decodeJSONWithLimit(w, r, &req, maxMessageBodyBytes); err != nil {
		writeDecodeError(w, err)
		return
	}

	clientMessageID, err := uuid.Parse(req.ClientMessageID)
	if err != nil || clientMessageID == uuid.Nil {
		http.Error(
			w,
			"client_message_id must be a non-empty UUID",
			http.StatusBadRequest,
		)
		return
	}

	message, err := h.messages.Send(r.Context(), domain.SendMessageInput{
		ChatID:          chatID,
		SenderID:        senderID,
		ClientMessageID: clientMessageID,
		Text:            req.Text,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "invalid message request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotChatMember):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrChatNotFound):
			http.Error(w, "chat not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrMessageConflict):
			http.Error(w, "client_message_id conflicts with existing message", http.StatusConflict)
		default:
			h.log.ErrorContext(
				r.Context(),
				"failed to send message",
				"request_id", middleware.GetRequestID(r.Context()),
				"error", err,
			)

			http.Error(w, "internal server error", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(MessageResponse{
		ID:              message.ID,
		ChatID:          message.ChatID,
		SenderID:        message.SenderID,
		ClientMessageID: message.ClientMessageID,
		Text:            message.Text,
		Seq:             message.Seq,
		CreatedAt:       message.CreatedAt,
	})
}

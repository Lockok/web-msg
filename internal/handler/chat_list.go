package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

type ChatListResponse struct {
	Items      []ChatResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

func (h *ChatHandler) ListUserChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid query parameters", http.StatusBadRequest)
		return
	}

	if len(params["limit"]) > 1 || len(params["cursor"]) > 1 {
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

	cursor, err := decodeChatCursor(params.Get("cursor"))
	if err != nil {
		http.Error(w, "invalid chat cursor", http.StatusBadRequest)
		return
	}

	page, err := h.chats.ListUserChats(r.Context(), userID, limit, cursor)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, "invalid chat list request", http.StatusBadRequest)
			return
		}

		h.writeChatListInternalError(w, r, err)
		return
	}

	response := ChatListResponse{
		Items: make([]ChatResponse, 0, len(page.Items)),
	}

	for _, chat := range page.Items {
		response.Items = append(response.Items, ChatResponse{
			ID:        chat.ID,
			Type:      chat.Type,
			Name:      chat.Name,
			CreatedAt: chat.CreatedAt,
		})
	}

	if page.NextCursor != nil {
		encoded, err := encodeChatCursor(page.NextCursor)
		if err != nil {
			// Некорректный курсор из service — внутренняя ошибка,
			// а не ошибка запроса клиента.
			h.writeChatListInternalError(w, r, err)
			return
		}

		response.NextCursor = &encoded
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *ChatHandler) writeChatListInternalError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	h.log.ErrorContext(
		r.Context(),
		"failed to list user chats",
		"request_id", middleware.GetRequestID(r.Context()),
		"error", err,
	)

	http.Error(w, "internal server error", http.StatusInternalServerError)
}

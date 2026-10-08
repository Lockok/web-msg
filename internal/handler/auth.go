package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type UserService interface{
	Register(ctx context.Context, username string, password string) (*domain.User, error)
}

type Handler struct{
	userService UserService
}

func NewHandler(userService UserService) *Handler {
	return &Handler{
		userService: userService,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	ID uuid.UUID	`json:"id"`
	Username string `json:"username"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.userService.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, "failed to register user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	
	_ = json.NewEncoder(w).Encode(RegisterResponse{
		ID: user.ID,
		Username: user.Username,
	})
}
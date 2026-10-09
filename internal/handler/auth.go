package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/google/uuid"
)

type UserService interface {
	Register(ctx context.Context, username string, password string) (*domain.User, error)
	Login(ctx context.Context, username string, password string) (*domain.User, error)
}

type Handler struct {
	userService UserService
	jwtManager  *auth.JWTManager
}

func NewHandler(userService UserService, jwtManager *auth.JWTManager) *Handler {
	return &Handler{
		userService: userService,
		jwtManager:  jwtManager,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
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
		if errors.Is(err, domain.ErrUsernameExists) {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}

		http.Error(w, "failed to register user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.userService.Login(
		r.Context(),
		req.Username,
		req.Password,
	)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		http.Error(w, "failed to login", http.StatusInternalServerError)
		return
	}

	token, err := h.jwtManager.Generate(user.ID)
	if err != nil {
		http.Error(w, "failed to create access token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   900,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"user_id": userID.String(),
	})
}

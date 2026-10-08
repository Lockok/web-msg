package server

import (
	"net/http"

	"github.com/Lockok/web-msg/internal/handler"
	"github.com/Lockok/web-msg/internal/health"
	"github.com/Lockok/web-msg/internal/service"
	"github.com/go-chi/chi"
)

func New(userService service.UserService) http.Handler {
	router := chi.NewRouter()

	healthHandler := health.NewHandler()
	router.Get("/health", healthHandler.Check)

	authHandler := handler.NewHandler(&userService)
	router.Post("/register", authHandler.Register)
	router.Post("/login", authHandler.Login)

	return router
}

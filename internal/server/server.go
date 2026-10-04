package server

import (
	"net/http"

	"github.com/Lockok/web-msg/internal/health"
	"github.com/go-chi/chi"
)

func New() http.Handler {
	router := chi.NewRouter()

	healthHandler := health.NewHandler()

	router.Get("/health", healthHandler.Check)

	return router
}
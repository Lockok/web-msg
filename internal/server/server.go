package server

import (
	"log/slog"
	"net/http"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/handler"
	"github.com/Lockok/web-msg/internal/health"
	"github.com/Lockok/web-msg/internal/middleware"
	"github.com/Lockok/web-msg/internal/service"
	"github.com/go-chi/chi"
)

func New(userService service.UserService, chatService *service.ChatService, messageService *service.MessageService, jwtManager *auth.JWTManager, log *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recovery(log))

	healthHandler := health.NewHandler()
	router.Get("/health", healthHandler.Check)

	authHandler := handler.NewHandler(&userService, jwtManager)
	router.Post("/register", authHandler.Register)
	router.Post("/login", authHandler.Login)
	router.With(middleware.Auth(jwtManager)).Get("/me", authHandler.Me)

	chatHandler := handler.NewChatHandler(chatService, log)
	router.With(middleware.Auth(jwtManager)).Post("/chats/private", chatHandler.GetOrCreatePrivateChat)
	router.With(middleware.Auth(jwtManager)).Get("/chats", chatHandler.ListUserChats)

	messageHandler := handler.NewMessageHandler(messageService, log)
	router.With(middleware.Auth(jwtManager)).Post("/chats/{chat_id}/messages", messageHandler.Send)
	router.With(middleware.Auth(jwtManager)).Get("/chats/{chat_id}/messages", messageHandler.ListChatMessages)

	webSocketHandler := handler.NewWebSocketHandler(jwtManager)
	router.Get("/ws", webSocketHandler.Connect)

	return router
}

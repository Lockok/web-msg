package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/config"
	"github.com/Lockok/web-msg/internal/logger"
	"github.com/Lockok/web-msg/internal/postgres"
	"github.com/Lockok/web-msg/internal/repository"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/Lockok/web-msg/internal/server"
	"github.com/Lockok/web-msg/internal/service"
)

func main() {
	cfg := config.Load()

	logger := slogger.New()

	logger.Info("starting server")

	pool, err := postgres.ConnectDB(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return
	}
	defer pool.Close()

	logger.Info("database connected")

	queries := db.New(pool)

	userRepository := repository.NewUserRepository(queries)

	userService := service.NewUserService(userRepository)

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	router := server.New(*userService, jwtManager, logger)

	log.Printf("Server is running on port: %s", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

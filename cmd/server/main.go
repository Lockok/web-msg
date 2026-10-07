package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Lockok/web-msg/internal/config"
	"github.com/Lockok/web-msg/internal/logger"
	"github.com/Lockok/web-msg/internal/postgres"
	"github.com/Lockok/web-msg/internal/server"
)

func main() {
	cfg := config.Load()

	logger := slogger.New()

	logger.Info("starting server")

	db, err := postgres.ConnectDB(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return
	}
	defer db.Close()

	logger.Info("database connected")
	
	router := server.New()

	log.Printf("Server is running on port: %s", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

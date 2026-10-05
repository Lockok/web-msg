package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Lockok/web-msg/internal/config"
	"github.com/Lockok/web-msg/internal/postgres"
	"github.com/Lockok/web-msg/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := postgres.ConnectDB(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close(context.Background())

	log.Println("database connected")

	router := server.New()
	
	log.Printf("Server is running on port: %s", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
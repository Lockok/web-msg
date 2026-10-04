package main

import (
	"log"
	"net/http"

	"github.com/Lockok/web-msg/internal/config"
	"github.com/Lockok/web-msg/internal/server"
)

func main() {
	cfg := config.Load()

	router := server.New()
	
	log.Printf("Server is running on port: %s", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
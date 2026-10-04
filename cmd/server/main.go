package main

import (
	"log"
	"net/http"

	"github.com/Lockok/web-msg/internal/server"
)

func main() {
	router := server.New()
	
	log.Println("Server is running on port 8080")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
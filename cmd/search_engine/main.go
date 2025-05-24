package main

import (
	"log"

	"github.com/gcbaptista/go-search-engine/api"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the search engine
	dataDir := "./search_data" // Define a data directory
	log.Printf("Using data directory: %s", dataDir)
	searchEngine := engine.NewEngine(dataDir) // Pass dataDir to the engine

	// Initialize Gin router
	router := gin.Default()

	// Setup API routes
	// We'll pass the searchEngine to our API handlers
	api.SetupRoutes(router, searchEngine)

	// Start the server
	port := "8080" // Default port
	log.Printf("Starting server on port %s...", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

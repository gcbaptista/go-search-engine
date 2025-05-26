package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gcbaptista/go-search-engine/api"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gin-gonic/gin"
)

func main() {
	// Define command-line flags
	var (
		help    = flag.Bool("help", false, "Show help message")
		version = flag.Bool("version", false, "Show version information")
		port    = flag.String("port", "8080", "Port to run the server on")
		dataDir = flag.String("data-dir", "./search_data", "Directory to store search data")
	)

	flag.Parse()

	// Handle help flag
	if *help {
		fmt.Printf("Go Search Engine - A high-performance search engine with typo tolerance\n\n")
		fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s                          # Start server on default port 8080\n", os.Args[0])
		fmt.Printf("  %s --port 9000              # Start server on port 9000\n", os.Args[0])
		fmt.Printf("  %s --data-dir /tmp/search   # Use custom data directory\n", os.Args[0])
		return
	}

	// Handle version flag
	if *version {
		fmt.Printf("Go Search Engine v1.0.0\n")
		fmt.Printf("Enhanced with typo tolerance, async operations, and analytics\n")
		return
	}

	// Initialize the search engine
	log.Printf("Using data directory: %s", *dataDir)
	searchEngine := engine.NewEngine(*dataDir)

	// Initialize Gin router
	router := gin.Default()

	// Setup API routes
	api.SetupRoutes(router, searchEngine)

	// Start the server
	log.Printf("Starting server on port %s...", *port)
	if err := router.Run(":" + *port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

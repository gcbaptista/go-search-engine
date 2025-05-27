package api

import (
	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/internal/analytics"
	"github.com/gcbaptista/go-search-engine/services"
)

// API holds dependencies for API handlers, primarily the search engine manager.
type API struct {
	engine    services.IndexManager
	analytics *analytics.Service
}

// NewAPI creates a new API handler structure.
func NewAPI(engine services.IndexManager) *API {
	return &API{
		engine:    engine,
		analytics: analytics.NewService(engine),
	}
}

// SetupRoutes defines all the API routes for the search engine.
func SetupRoutes(router *gin.Engine, engine services.IndexManager) {
	// Add middleware
	router.Use(CORSMiddleware())
	router.Use(RequestSizeLimitMiddleware(500 << 20)) // 500 MB limit

	apiHandler := NewAPI(engine)

	// Health check route
	router.GET("/health", apiHandler.HealthCheckHandler)

	// Analytics route
	router.GET("/analytics", apiHandler.GetAnalyticsHandler)

	// Job management routes
	jobRoutes := router.Group("/jobs")
	{
		jobRoutes.GET("/:jobId", apiHandler.GetJobHandler)         // Get job status by ID
		jobRoutes.GET("/metrics", apiHandler.GetJobMetricsHandler) // Get job performance metrics
	}

	// Index management routes
	indexRoutes := router.Group("/indexes")
	{
		indexRoutes.POST("", apiHandler.CreateIndexHandler)                              // Create a new index
		indexRoutes.GET("", apiHandler.ListIndexesHandler)                               // List all indexes
		indexRoutes.GET("/:indexName", apiHandler.GetIndexHandler)                       // Get specific index details (e.g., settings)
		indexRoutes.DELETE("/:indexName", apiHandler.DeleteIndexHandler)                 // Delete an index
		indexRoutes.PATCH("/:indexName/settings", apiHandler.UpdateIndexSettingsHandler) // Update index settings
		indexRoutes.POST("/:indexName/rename", apiHandler.RenameIndexHandler)            // Rename an index
		indexRoutes.GET("/:indexName/stats", apiHandler.GetIndexStatsHandler)            // Get index statistics
		indexRoutes.GET("/:indexName/jobs", apiHandler.ListJobsHandler)                  // List jobs for an index

		// Document management routes per index
		docRoutes := indexRoutes.Group("/:indexName/documents")
		{
			docRoutes.PUT("", apiHandler.AddDocumentsHandler)                  // Add/Update documents
			docRoutes.GET("", apiHandler.GetDocumentsHandler)                  // List documents with pagination
			docRoutes.DELETE("", apiHandler.DeleteAllDocumentsHandler)         // Delete all documents
			docRoutes.GET("/:documentId", apiHandler.GetDocumentHandler)       // Get specific document
			docRoutes.DELETE("/:documentId", apiHandler.DeleteDocumentHandler) // Delete specific document
		}

		// Search routes per index
		indexRoutes.POST("/:indexName/_search", apiHandler.SearchHandler)
		indexRoutes.POST("/:indexName/_multi_search", apiHandler.MultiSearchHandler)
	}
}

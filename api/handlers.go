package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/analytics"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
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

		// Search route per index
		indexRoutes.POST("/:indexName/_search", apiHandler.SearchHandler)
	}
}

// CreateIndexHandler handles the request to create a new index.
// Request Body: config.IndexSettings
func (api *API) CreateIndexHandler(c *gin.Context) {
	var settings config.IndexSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if settings.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Index name is required"})
		return
	}

	// Default ranking if not provided
	if len(settings.RankingCriteria) == 0 {
		// Use search service default (score-based) ranking
	}

	// Create index asynchronously
	var jobID string
	var err error
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.CreateIndexAsync(settings)
	} else {
		err = api.engine.CreateIndex(settings)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create index: " + err.Error()})
		return
	}

	if jobID != "" {
		// Async response with job ID
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "accepted",
			"message": "Index creation started for '" + settings.Name + "'",
			"job_id":  jobID,
		})
	} else {
		c.JSON(http.StatusCreated, gin.H{"message": "Index '" + settings.Name + "' created successfully"})
	}
}

// AddDocumentsHandler handles adding/updating documents in an index.
func (api *API) AddDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Read the raw JSON data first
	var rawData interface{}
	if err := c.ShouldBindJSON(&rawData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	var docs []model.Document

	// Check if the raw data is a slice (array) or a single object
	if dataSlice, isSlice := rawData.([]interface{}); isSlice {
		// Handle array of documents
		docs = make([]model.Document, len(dataSlice))
		for i, item := range dataSlice {
			if docMap, isMap := item.(map[string]interface{}); isMap {
				docs[i] = model.Document(docMap)
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d is not a valid object", i)})
				return
			}
		}
	} else if docMap, isMap := rawData.(map[string]interface{}); isMap {
		// Handle single document
		docs = []model.Document{model.Document(docMap)}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. Expecting a document object or an array of documents"})
		return
	}

	if len(docs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No documents provided"})
		return
	}

	// Process documents: ensure documentID is present and is a valid string.
	for i := range docs {
		docMap := docs[i] // docs[i] is a map[string]interface{}

		// Handle documentID: must be present and valid (no auto-generation)
		uuidVal, uuidExists := docMap["documentID"]
		if !uuidExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d must have a 'documentID' field", i)})
			return
		}

		var finalDocumentID string
		switch u := uuidVal.(type) {
		case string:
			if strings.TrimSpace(u) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d has empty or whitespace-only documentID string", i)})
				return
			}
			finalDocumentID = strings.TrimSpace(u)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d has documentID with unexpected type: %T (expected string)", i, uuidVal)})
			return
		}
		docMap["documentID"] = finalDocumentID // Ensure the map has a clean string for the indexing service.

		// documentID is the only required field; all others depend on index configuration
	}

	// Add documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.AddDocumentsAsync(indexName, docs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document addition: " + err.Error()})
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":         "accepted",
			"message":        fmt.Sprintf("Document addition started for index '%s' (%d documents)", indexName, len(docs)),
			"job_id":         jobID,
			"document_count": len(docs),
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.AddDocuments(docs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add documents to index '" + indexName + "': " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d document(s) added/updated in index '%s'", len(docs), indexName)})
	}
}

// DeleteAllDocumentsHandler handles the request to delete all documents from an index.
func (api *API) DeleteAllDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Delete all documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteAllDocumentsAsync(indexName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document deletion: " + err.Error()})
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "accepted",
			"message": fmt.Sprintf("Document deletion started for index '%s'", indexName),
			"job_id":  jobID,
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.DeleteAllDocuments()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all documents from index '" + indexName + "': " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "All documents deleted from index '" + indexName + "'"})
	}
}

// SearchRequest defines the structure for search queries.
// It's slightly different from services.SearchQuery to accommodate JSON binding for filters.
type SearchRequest struct {
	Query                    string                 `json:"query"`
	Filters                  map[string]interface{} `json:"filters"`
	Page                     int                    `json:"page"`
	PageSize                 int                    `json:"page_size"`
	RestrictSearchableFields []string               `json:"restrict_searchable_fields,omitempty"`
	RetrivableFields         []string               `json:"retrivable_fields,omitempty"`
	MinWordSizeFor1Typo      *int                   `json:"min_word_size_for_1_typo,omitempty"`  // Optional: override index setting for minimum word size for 1 typo
	MinWordSizeFor2Typos     *int                   `json:"min_word_size_for_2_typos,omitempty"` // Optional: override index setting for minimum word size for 2 typos
}

// SearchHandler handles search requests to an index.
// Request Body: SearchRequest (similar to services.SearchQuery but adapted for JSON)
func (api *API) SearchHandler(c *gin.Context) {
	startTime := time.Now()
	indexName := c.Param("indexName")

	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid search request body: " + err.Error()})
		return
	}

	searchQuery := services.SearchQuery{
		QueryString:              req.Query,
		Filters:                  req.Filters, // Direct pass-through for now
		Page:                     req.Page,
		PageSize:                 req.PageSize,
		RestrictSearchableFields: req.RestrictSearchableFields,
		RetrivableFields:         req.RetrivableFields,
		MinWordSizeFor1Typo:      req.MinWordSizeFor1Typo,
		MinWordSizeFor2Typos:     req.MinWordSizeFor2Typos,
	}

	results, err := indexAccessor.Search(searchQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error performing search on index '" + indexName + "': " + err.Error()})
		return
	}

	// Track analytics event
	responseTime := time.Since(startTime)
	searchType := api.determineSearchType(req)

	event := model.SearchEvent{
		IndexName:    indexName,
		Query:        req.Query,
		SearchType:   searchType,
		ResponseTime: responseTime,
		ResultCount:  results.Total,
		Filters:      req.Filters,
	}

	// Track the event asynchronously to avoid slowing down the response
	go func() {
		if err := api.analytics.TrackSearchEvent(event); err != nil {
			log.Printf("Warning: Failed to track search event: %v", err)
		}
	}()

	c.JSON(http.StatusOK, results)
}

// determineSearchType determines the type of search based on the request
func (api *API) determineSearchType(req SearchRequest) string {
	if len(req.Filters) > 0 {
		return "filtered"
	}
	if strings.Contains(req.Query, "*") || strings.Contains(req.Query, "?") {
		return "wildcard"
	}
	if req.Query == "" {
		return "filtered" // Empty query with filters
	}

	// Check if it might be fuzzy (simplified heuristic)
	if len(strings.Fields(req.Query)) == 1 && len(req.Query) > 3 {
		return "fuzzy_search"
	}

	return "exact_match"
}

// ListIndexesHandler lists all available indexes.
func (api *API) ListIndexesHandler(c *gin.Context) {
	names := api.engine.ListIndexes()
	c.JSON(http.StatusOK, gin.H{"indexes": names, "count": len(names)})
}

// GetIndexHandler retrieves details about a specific index (its settings).
func (api *API) GetIndexHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}
	c.JSON(http.StatusOK, indexAccessor.Settings())
}

// DeleteIndexHandler handles deleting an index.
func (api *API) DeleteIndexHandler(c *gin.Context) {
	indexName := c.Param("indexName")

	// Delete index asynchronously
	var jobID string
	var err error
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteIndexAsync(indexName)
	} else {
		err = api.engine.DeleteIndex(indexName)
	}

	if err != nil {
		// Check if the error indicates the index was not found
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
			return
		}
		// For other errors (file system errors, etc.), return internal server error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete index '" + indexName + "': " + err.Error()})
		return
	}

	if jobID != "" {
		// Async response with job ID
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "accepted",
			"message": "Index deletion started for '" + indexName + "'",
			"job_id":  jobID,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Index '" + indexName + "' deleted successfully"})
	}
}

// RenameIndexRequest defines the structure for renaming an index
type RenameIndexRequest struct {
	NewName string `json:"new_name" binding:"required"`
}

// RenameIndexHandler handles requests to rename an index
func (api *API) RenameIndexHandler(c *gin.Context) {
	oldName := c.Param("indexName")

	var req RenameIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if req.NewName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new_name is required and cannot be empty"})
		return
	}

	// Validate new name
	if strings.TrimSpace(req.NewName) != req.NewName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new_name cannot have leading or trailing whitespace"})
		return
	}

	// Rename index asynchronously
	var jobID string
	var err error
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.RenameIndexAsync(oldName, req.NewName)
	} else {
		err = api.engine.RenameIndex(oldName, req.NewName)
	}

	if err != nil {
		// Determine the appropriate HTTP status based on the error
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + oldName + "' not found"})
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "same") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// For other errors (file system errors, etc.), return internal server error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rename index: " + err.Error()})
		return
	}

	if jobID != "" {
		// Async response with job ID
		c.JSON(http.StatusAccepted, gin.H{
			"status":   "accepted",
			"message":  fmt.Sprintf("Index rename started: '%s' -> '%s'", oldName, req.NewName),
			"job_id":   jobID,
			"old_name": oldName,
			"new_name": req.NewName,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message":  "Index renamed successfully",
			"old_name": oldName,
			"new_name": req.NewName,
		})
	}
}

// IndexSettingsUpdate defines the structure for updating index settings
type IndexSettingsUpdate struct {
	FieldsWithoutPrefixSearch *[]string                  `json:"fields_without_prefix_search,omitempty"` // Use []string, not *[]string, to allow sending an empty list to clear
	NoTypoToleranceFields     *[]string                  `json:"no_typo_tolerance_fields,omitempty"`     // Use []string to allow sending an empty list to clear
	DistinctField             *string                    `json:"distinct_field,omitempty"`               // Use pointer to distinguish between empty string and not provided
	SearchableFields          *[]string                  `json:"searchable_fields,omitempty"`            // Fields that can be searched, in priority order
	FilterableFields          *[]string                  `json:"filterable_fields,omitempty"`            // Fields that can be used in filters
	RankingCriteria           *[]config.RankingCriterion `json:"ranking_criteria,omitempty"`             // Ranking criteria for search results
	MinWordSizeFor1Typo       *int                       `json:"min_word_size_for_1_typo,omitempty"`     // Minimum word length to allow 1 typo
	MinWordSizeFor2Typos      *int                       `json:"min_word_size_for_2_typos,omitempty"`    // Minimum word length to allow 2 typos
}

// UpdateIndexSettingsHandler handles requests to update index settings
func (api *API) UpdateIndexSettingsHandler(c *gin.Context) {
	indexName := c.Param("indexName")

	settings, err := api.engine.GetIndexSettings(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found or error getting settings: " + err.Error()})
		return
	}

	// Read raw request first to check for key presence
	rawRequest := make(map[string]interface{})
	if err := c.ShouldBindJSON(&rawRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	originalSettings := settings // Keep a copy to detect changes that require reindexing
	updated := false
	requiresReindexing := false

	// Handle searchable_fields (CORE SETTING - requires reindexing)
	if fieldValue, keyExists := rawRequest["searchable_fields"]; keyExists {
		if fieldValue == nil {
			settings.SearchableFields = []string{}
		} else if fieldSlice, isSlice := fieldValue.([]interface{}); isSlice {
			stringSlice := make([]string, len(fieldSlice))
			for i, v := range fieldSlice {
				if str, isStr := v.(string); isStr {
					stringSlice[i] = str
				}
			}
			settings.SearchableFields = stringSlice
		}
		if !slicesEqual(originalSettings.SearchableFields, settings.SearchableFields) {
			requiresReindexing = true
		}
		updated = true
	}

	// Handle filterable_fields (CORE SETTING - may require reindexing)
	if fieldValue, keyExists := rawRequest["filterable_fields"]; keyExists {
		if fieldValue == nil {
			settings.FilterableFields = []string{}
		} else if fieldSlice, isSlice := fieldValue.([]interface{}); isSlice {
			stringSlice := make([]string, len(fieldSlice))
			for i, v := range fieldSlice {
				if str, isStr := v.(string); isStr {
					stringSlice[i] = str
				}
			}
			settings.FilterableFields = stringSlice
		}
		if !slicesEqual(originalSettings.FilterableFields, settings.FilterableFields) {
			requiresReindexing = true
		}
		updated = true
	}

	// Handle ranking_criteria (CORE SETTING - affects search results)
	if fieldValue, keyExists := rawRequest["ranking_criteria"]; keyExists {
		if fieldValue == nil {
			settings.RankingCriteria = []config.RankingCriterion{}
		} else if criteriaSlice, isSlice := fieldValue.([]interface{}); isSlice {
			rankingCriteria := make([]config.RankingCriterion, len(criteriaSlice))
			for i, v := range criteriaSlice {
				if criterionMap, isMap := v.(map[string]interface{}); isMap {
					var criterion config.RankingCriterion
					if field, hasField := criterionMap["field"].(string); hasField {
						criterion.Field = field
					}
					if order, hasOrder := criterionMap["order"].(string); hasOrder {
						criterion.Order = order
					}
					rankingCriteria[i] = criterion
				}
			}
			settings.RankingCriteria = rankingCriteria
		}
		if !rankingCriteriaEqual(originalSettings.RankingCriteria, settings.RankingCriteria) {
			requiresReindexing = true
		}
		updated = true
	}

	// Handle min_word_size_for_1_typo (CORE SETTING - requires reindexing)
	if fieldValue, keyExists := rawRequest["min_word_size_for_1_typo"]; keyExists {
		if num, isNum := fieldValue.(float64); isNum {
			intVal := int(num)
			if originalSettings.MinWordSizeFor1Typo != intVal {
				requiresReindexing = true
			}
			settings.MinWordSizeFor1Typo = intVal
		}
		updated = true
	}

	// Handle min_word_size_for_2_typos (CORE SETTING - requires reindexing)
	if fieldValue, keyExists := rawRequest["min_word_size_for_2_typos"]; keyExists {
		if num, isNum := fieldValue.(float64); isNum {
			intVal := int(num)
			if originalSettings.MinWordSizeFor2Typos != intVal {
				requiresReindexing = true
			}
			settings.MinWordSizeFor2Typos = intVal
		}
		updated = true
	}

	// Handle fields_without_prefix_search (field-level setting)
	if fieldValue, keyExists := rawRequest["fields_without_prefix_search"]; keyExists {
		if fieldValue == nil {
			settings.FieldsWithoutPrefixSearch = []string{}
		} else if fieldSlice, isSlice := fieldValue.([]interface{}); isSlice {
			stringSlice := make([]string, len(fieldSlice))
			for i, v := range fieldSlice {
				if str, isStr := v.(string); isStr {
					stringSlice[i] = str
				}
			}
			settings.FieldsWithoutPrefixSearch = stringSlice
		}
		updated = true
	}

	// Handle no_typo_tolerance_fields (field-level setting)
	if fieldValue, keyExists := rawRequest["no_typo_tolerance_fields"]; keyExists {
		if fieldValue == nil {
			settings.NoTypoToleranceFields = []string{}
		} else if fieldSlice, isSlice := fieldValue.([]interface{}); isSlice {
			stringSlice := make([]string, len(fieldSlice))
			for i, v := range fieldSlice {
				if str, isStr := v.(string); isStr {
					stringSlice[i] = str
				}
			}
			settings.NoTypoToleranceFields = stringSlice
		}
		updated = true
	}

	// Handle distinct_field (field-level setting)
	if fieldValue, keyExists := rawRequest["distinct_field"]; keyExists {
		if fieldValue == nil {
			settings.DistinctField = ""
		} else if str, isStr := fieldValue.(string); isStr {
			settings.DistinctField = str
		}
		updated = true
	}

	if !updated {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid updatable fields provided or no changes detected"})
		return
	}

	// Validate field names to prevent conflicts with filter operators
	if conflicts := settings.ValidateFieldNames(); len(conflicts) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Field name validation failed",
			"conflicts": conflicts,
		})
		return
	}

	// Automatically determines if reindexing is needed
	var jobID string
	if engineWithAsyncReindex, ok := api.engine.(services.IndexManagerWithAsyncReindex); ok {
		jobID, err = engineWithAsyncReindex.UpdateIndexSettingsWithAsyncReindex(indexName, settings)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async settings update: " + err.Error()})
			return
		}
	} else {
		err = api.engine.UpdateIndexSettings(indexName, settings)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update index settings: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":   "Settings updated successfully for index '" + indexName + "'",
			"reindexed": requiresReindexing,
		})
		return
	}

	// Return async response with job ID
	c.JSON(http.StatusAccepted, gin.H{
		"status":              "accepted",
		"message":             "Settings update started for index '" + indexName + "' (search-time settings update)",
		"job_id":              jobID,
		"reindexing_required": requiresReindexing,
	})
}

// Helper function to compare string slices
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Helper function to compare ranking criteria slices
func rankingCriteriaEqual(a, b []config.RankingCriterion) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Field != b[i].Field || a[i].Order != b[i].Order {
			return false
		}
	}
	return true
}

// HealthCheckHandler provides a simple health check endpoint
func (api *API) HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "go-search-engine",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

// GetIndexStatsHandler returns statistics for a specific index
func (api *API) GetIndexStatsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	settings := indexAccessor.Settings()

	// Get document count from the document store
	documentCount := 0
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if instance, err := concreteEngine.GetIndex(indexName); err == nil {
			if engineInstance, ok := instance.(*engine.IndexInstance); ok {
				documentCount = len(engineInstance.DocumentStore.Docs)
			}
		}
	}

	stats := gin.H{
		"name":              settings.Name,
		"document_count":    documentCount,
		"searchable_fields": settings.SearchableFields,
		"filterable_fields": settings.FilterableFields,
		"typo_settings": gin.H{
			"min_word_size_for_1_typo":  settings.MinWordSizeFor1Typo,
			"min_word_size_for_2_typos": settings.MinWordSizeFor2Typos,
		},
		"field_settings": gin.H{
			"fields_without_prefix_search": settings.FieldsWithoutPrefixSearch,
			"no_typo_tolerance_fields":     settings.NoTypoToleranceFields,
			"distinct_field":               settings.DistinctField,
		},
	}

	c.JSON(http.StatusOK, stats)
}

// DocumentListRequest defines the structure for document listing requests
type DocumentListRequest struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// GetDocumentsHandler lists documents in an index with pagination
func (api *API) GetDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	var req DocumentListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}

	// Set defaults
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100 // Maximum page size
	}

	documents := []model.Document{}
	totalCount := 0

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if instance, err := concreteEngine.GetIndex(indexName); err == nil {
			if engineInstance, ok := instance.(*engine.IndexInstance); ok {
				allDocs := engineInstance.DocumentStore.Docs
				totalCount = len(allDocs)

				// Calculate pagination
				startIndex := (req.Page - 1) * req.PageSize
				endIndex := startIndex + req.PageSize

				i := 0
				for _, doc := range allDocs {
					if i >= startIndex && i < endIndex {
						documents = append(documents, doc)
					}
					i++
					if i >= endIndex {
						break
					}
				}
			}
		}
	}

	response := gin.H{
		"documents": documents,
		"total":     totalCount,
		"page":      req.Page,
		"page_size": req.PageSize,
		"pages":     (totalCount + req.PageSize - 1) / req.PageSize,
	}

	c.JSON(http.StatusOK, response)
}

// GetDocumentHandler retrieves a specific document by ID
func (api *API) GetDocumentHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	documentId := c.Param("documentId")

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	var document model.Document
	found := false

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if instance, err := concreteEngine.GetIndex(indexName); err == nil {
			if engineInstance, ok := instance.(*engine.IndexInstance); ok {
				for _, doc := range engineInstance.DocumentStore.Docs {
					if docID, exists := doc.GetDocumentID(); exists && docID == documentId {
						document = doc
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
		return
	}

	c.JSON(http.StatusOK, document)
}

// DeleteDocumentHandler deletes a specific document by ID
func (api *API) DeleteDocumentHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	documentId := c.Param("documentId")

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Delete document asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteDocumentAsync(indexName, documentId)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document deletion: " + err.Error()})
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":      "accepted",
			"message":     fmt.Sprintf("Document deletion started for document '%s' in index '%s'", documentId, indexName),
			"job_id":      jobID,
			"document_id": documentId,
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.DeleteDocument(documentId)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document '" + documentId + "' from index '" + indexName + "': " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Document '" + documentId + "' deleted from index '" + indexName + "'"})
	}
}

// GetAnalyticsHandler handles the request to get analytics data
func (api *API) GetAnalyticsHandler(c *gin.Context) {
	dashboard, err := api.analytics.GetDashboardData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve analytics data: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// GetJobHandler handles requests to get job status by ID
func (api *API) GetJobHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	if jobManager, ok := api.engine.(services.JobManager); ok {
		job, err := jobManager.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, job)
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job management not supported by this engine"})
	}
}

// ListJobsHandler handles requests to list jobs for an index
func (api *API) ListJobsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	statusParam := c.Query("status")

	var statusFilter *model.JobStatus
	if statusParam != "" {
		status := model.JobStatus(statusParam)
		statusFilter = &status
	}

	if jobManager, ok := api.engine.(services.JobManager); ok {
		jobs := jobManager.ListJobs(indexName, statusFilter)
		c.JSON(http.StatusOK, gin.H{
			"jobs":       jobs,
			"index_name": indexName,
			"total":      len(jobs),
		})
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job management not supported by this engine"})
	}
}

// GetJobMetricsHandler handles requests to get job performance metrics
func (api *API) GetJobMetricsHandler(c *gin.Context) {
	if engineWithMetrics, ok := api.engine.(*engine.Engine); ok {
		// Get metrics (already returns a copy without mutex)
		metrics := engineWithMetrics.GetJobMetrics()

		// Add computed metrics
		response := gin.H{
			"metrics":          metrics,
			"success_rate":     engineWithMetrics.GetJobSuccessRate(),
			"current_workload": engineWithMetrics.GetCurrentWorkload(),
		}

		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job metrics not supported by this engine"})
	}
}

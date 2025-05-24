package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gin-gonic/gin"
)

// API holds dependencies for API handlers, primarily the search engine manager.
type API struct {
	engine services.IndexManager
}

// NewAPI creates a new API handler structure.
func NewAPI(engine services.IndexManager) *API {
	return &API{engine: engine}
}

// SetupRoutes defines all the API routes for the search engine.
func SetupRoutes(router *gin.Engine, engine services.IndexManager) {
	apiHandler := NewAPI(engine)

	// Health check route
	router.GET("/health", apiHandler.HealthCheckHandler)

	// Index management routes
	indexRoutes := router.Group("/indexes")
	{
		indexRoutes.POST("", apiHandler.CreateIndexHandler)                              // Create a new index
		indexRoutes.GET("", apiHandler.ListIndexesHandler)                               // List all indexes
		indexRoutes.GET("/:indexName", apiHandler.GetIndexHandler)                       // Get specific index details (e.g., settings)
		indexRoutes.DELETE("/:indexName", apiHandler.DeleteIndexHandler)                 // Delete an index
		indexRoutes.PATCH("/:indexName/settings", apiHandler.UpdateIndexSettingsHandler) // Update index settings
		indexRoutes.GET("/:indexName/stats", apiHandler.GetIndexStatsHandler)            // Get index statistics

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
		// Default to sorting by relevance score (implicit) and then by a common field like title or a specified default.
		// For now, let's assume if no ranking criteria are provided, the search service default (score-based) is sufficient.
		// Or, we can add a default like:
		// settings.RankingCriteria = []config.RankingCriterion{{"Field": "title", "Order": "asc"}}
		// However, the bootstrap script already provides ranking criteria. Let's keep it simple:
		// No explicit default ranking criteria added here; expect it from client or rely on search service defaults.
	}

	err := api.engine.CreateIndex(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create index: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Index '" + settings.Name + "' created successfully"})
}

// AddDocumentsHandler handles adding/updating documents in an index.
func (api *API) AddDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	indexAccessor, err := api.engine.GetIndex(indexName)
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

		// Note: In a truly schema-agnostic system, documentID is the only required field
		// All other fields are optional and depend on the index configuration (searchable_fields, filterable_fields)
	}

	err = indexAccessor.AddDocuments(docs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add documents to index '" + indexName + "': " + err.Error()})
		return
	}

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if err := concreteEngine.PersistIndexData(indexName); err != nil {
			log.Printf("Warning: Failed to persist data for index '%s' after adding documents: %v", indexName, err)
		}
	} else {
		log.Printf("Warning: Could not type assert IndexManager to Engine to persist data for index '%s'. Persistence skipped.", indexName)
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d document(s) added/updated in index '%s'", len(docs), indexName)})
}

// DeleteAllDocumentsHandler handles the request to delete all documents from an index.
func (api *API) DeleteAllDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	err = indexAccessor.DeleteAllDocuments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all documents from index '" + indexName + "': " + err.Error()})
		return
	}

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if err := concreteEngine.PersistIndexData(indexName); err != nil {
			log.Printf("Warning: Failed to persist data for index '%s' after deleting all documents: %v", indexName, err)
		}
	} else {
		log.Printf("Warning: Could not type assert IndexManager to Engine to persist data for index '%s'. Persistence skipped.", indexName)
	}

	c.JSON(http.StatusOK, gin.H{"message": "All documents deleted from index '" + indexName + "'"})
}

// SearchRequest defines the structure for search queries.
// It's slightly different from services.SearchQuery to accommodate JSON binding for filters.
type SearchRequest struct {
	Query    string                 `json:"query"`
	Filters  map[string]interface{} `json:"filters"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// SearchHandler handles search requests to an index.
// Request Body: SearchRequest (similar to services.SearchQuery but adapted for JSON)
func (api *API) SearchHandler(c *gin.Context) {
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
		QueryString: req.Query,
		Filters:     req.Filters, // Direct pass-through for now
		Page:        req.Page,
		PageSize:    req.PageSize,
	}

	results, err := indexAccessor.Search(searchQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error performing search on index '" + indexName + "': " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
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

// DeleteIndexHandler removes an index.
func (api *API) DeleteIndexHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	err := api.engine.DeleteIndex(indexName)
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
	c.JSON(http.StatusOK, gin.H{"message": "Index '" + indexName + "' deleted successfully"})
}

// IndexSettingsUpdate defines the structure for updating index settings
type IndexSettingsUpdate struct {
	FieldsWithoutPrefixSearch *[]string `json:"fields_without_prefix_search,omitempty"` // Use []string, not *[]string, to allow sending an empty list to clear
	NoTypoToleranceFields     *[]string `json:"no_typo_tolerance_fields,omitempty"`     // Use []string to allow sending an empty list to clear
	DistinctField             *string   `json:"distinct_field,omitempty"`               // Use pointer to distinguish between empty string and not provided
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

	updated := false

	// Handle fields_without_prefix_search
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

	// Handle no_typo_tolerance_fields
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

	// Handle distinct_field
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

	err = api.engine.UpdateIndexSettings(indexName, settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update index settings: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully for index '" + indexName + "'",
		"warning": "You may need to reindex your documents for changes to FieldsWithoutPrefixSearch to take full effect.",
	})
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

	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Delete the document
	err = indexAccessor.DeleteDocument(documentId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document '" + documentId + "' from index '" + indexName + "': " + err.Error()})
		return
	}

	// Persist the changes
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if err := concreteEngine.PersistIndexData(indexName); err != nil {
			log.Printf("Warning: Failed to persist data for index '%s' after deleting document '%s': %v", indexName, documentId, err)
		}
	} else {
		log.Printf("Warning: Could not type assert IndexManager to Engine to persist data for index '%s'. Persistence skipped.", indexName)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document '" + documentId + "' deleted from index '" + indexName + "'"})
}

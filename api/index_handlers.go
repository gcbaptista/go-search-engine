package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/services"
)

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

	// Default ranking if not provided - search service will use default (score-based) ranking

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
	NonTypoTolerantWords      *[]string                  `json:"non_typo_tolerant_words,omitempty"`      // Specific words that should never be typo-matched
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

	// Handle non_typo_tolerant_words (word-level setting)
	if fieldValue, keyExists := rawRequest["non_typo_tolerant_words"]; keyExists {
		if fieldValue == nil {
			settings.NonTypoTolerantWords = []string{}
		} else if fieldSlice, isSlice := fieldValue.([]interface{}); isSlice {
			stringSlice := make([]string, len(fieldSlice))
			for i, v := range fieldSlice {
				if str, isStr := v.(string); isStr {
					stringSlice[i] = str
				}
			}
			settings.NonTypoTolerantWords = stringSlice
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

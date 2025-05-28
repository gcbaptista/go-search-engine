package api

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	internalErrors "github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// SearchRequest defines the structure for search queries.
type SearchRequest struct {
	Query                    string            `json:"query"`
	Filters                  *services.Filters `json:"filters,omitempty"`
	Page                     int               `json:"page"`
	PageSize                 int               `json:"page_size"`
	RestrictSearchableFields []string          `json:"restrict_searchable_fields,omitempty"`
	RetrievableFields        []string          `json:"retrievable_fields,omitempty"`
	MinWordSizeFor1Typo      *int              `json:"min_word_size_for_1_typo,omitempty"`  // Optional: override index setting for minimum word size for 1 typo
	MinWordSizeFor2Typos     *int              `json:"min_word_size_for_2_typos,omitempty"` // Optional: override index setting for minimum word size for 2 typos
}

// MultiSearchRequest represents the JSON request for multi-search
type MultiSearchRequest struct {
	Queries  []NamedSearchRequest `json:"queries" binding:"required"`
	Page     int                  `json:"page,omitempty"`
	PageSize int                  `json:"page_size,omitempty"`
}

// NamedSearchRequest represents a single named search query in the request
type NamedSearchRequest struct {
	Name                     string            `json:"name" binding:"required"`
	Query                    string            `json:"query" binding:"required"`
	RestrictSearchableFields []string          `json:"restrict_searchable_fields,omitempty"`
	RetrievableFields        []string          `json:"retrievable_fields,omitempty"`
	Filters                  *services.Filters `json:"filters,omitempty"`
	MinWordSizeFor1Typo      *int              `json:"min_word_size_for_1_typo,omitempty"`
	MinWordSizeFor2Typos     *int              `json:"min_word_size_for_2_typos,omitempty"`
}

// SearchHandler handles search requests to an index.
// Request Body: SearchRequest (similar to services.SearchQuery but adapted for JSON)
func (api *API) SearchHandler(c *gin.Context) {
	startTime := time.Now()
	indexName := c.Param("indexName")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	var req SearchRequest

	// Bind JSON directly with error handling
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid request body: "+err.Error())
		return
	}

	searchQuery := services.SearchQuery{
		QueryString:              req.Query,
		Filters:                  req.Filters,
		Page:                     req.Page,
		PageSize:                 req.PageSize,
		RestrictSearchableFields: req.RestrictSearchableFields,
		RetrievableFields:        req.RetrievableFields,
		MinWordSizeFor1Typo:      req.MinWordSizeFor1Typo,
		MinWordSizeFor2Typos:     req.MinWordSizeFor2Typos,
	}

	results, err := indexAccessor.Search(searchQuery)
	if err != nil {
		SendSearchError(c, indexName, err)
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
	}

	// Track the event asynchronously to avoid slowing down the response
	go func() {
		if err := api.analytics.TrackSearchEvent(event); err != nil {
			log.Printf("Warning: Failed to track search event: %v", err)
		}
	}()

	c.JSON(http.StatusOK, results)
}

// MultiSearchHandler handles multi-query search requests to an index.
// Request Body: MultiSearchRequest
func (api *API) MultiSearchHandler(c *gin.Context) {
	startTime := time.Now()
	indexName := c.Param("indexName")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	indexAccessor, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	var req MultiSearchRequest

	// Bind JSON directly with error handling
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid request body: "+err.Error())
		return
	}

	// Validate that we have at least one query
	if len(req.Queries) == 0 {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "At least one query is required")
		return
	}

	// Validate query names are unique
	queryNames := make(map[string]bool)
	for _, namedQuery := range req.Queries {
		if namedQuery.Name == "" {
			SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "All queries must have a non-empty name")
			return
		}
		if queryNames[namedQuery.Name] {
			SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Query names must be unique: '"+namedQuery.Name+"' appears multiple times")
			return
		}
		queryNames[namedQuery.Name] = true
	}

	// Convert API request to service request
	multiSearchQuery := services.MultiSearchQuery{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// Convert named search requests
	for _, namedReq := range req.Queries {
		namedQuery := services.NamedSearchQuery{
			Name:                     namedReq.Name,
			Query:                    namedReq.Query,
			RestrictSearchableFields: namedReq.RestrictSearchableFields,
			RetrievableFields:        namedReq.RetrievableFields,
			Filters:                  namedReq.Filters,
			MinWordSizeFor1Typo:      namedReq.MinWordSizeFor1Typo,
			MinWordSizeFor2Typos:     namedReq.MinWordSizeFor2Typos,
		}
		multiSearchQuery.Queries = append(multiSearchQuery.Queries, namedQuery)
	}

	results, err := indexAccessor.MultiSearch(multiSearchQuery)
	if err != nil {
		SendSearchError(c, indexName, err)
		return
	}

	// Track analytics events for each individual query
	responseTime := time.Since(startTime)
	for queryName, result := range results.Results {
		// Find the original request for this query to get the query string
		var originalQuery string
		for _, namedReq := range req.Queries {
			if namedReq.Name == queryName {
				originalQuery = namedReq.Query
				break
			}
		}

		event := model.SearchEvent{
			IndexName:    indexName,
			Query:        originalQuery,
			SearchType:   "multi_search",
			ResponseTime: responseTime,
			ResultCount:  result.Total,
		}

		// Track the event asynchronously
		go func(e model.SearchEvent) {
			if err := api.analytics.TrackSearchEvent(e); err != nil {
				log.Printf("Warning: Failed to track search event: %v", err)
			}
		}(event)
	}

	c.JSON(http.StatusOK, results)
}

// determineSearchType determines the type of search based on the request
func (api *API) determineSearchType(req SearchRequest) string {
	if req.Filters != nil {
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

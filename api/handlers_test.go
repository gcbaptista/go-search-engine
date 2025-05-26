package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gin-gonic/gin"
)

// Global registry to track test directories for cleanup
var (
	testDirs   []string
	testDirsMu sync.Mutex
)

func setupTestEngine() *engine.Engine {
	// Use unique test directory for each test run
	testDir := fmt.Sprintf("./test_data_%d", time.Now().UnixNano())

	// Register directory for cleanup
	testDirsMu.Lock()
	testDirs = append(testDirs, testDir)
	testDirsMu.Unlock()

	return engine.NewEngine(testDir)
}

func setupTestRouter(eng *engine.Engine) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router, eng)
	return router
}

func TestCreateIndexHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid index creation",
			requestBody: config.IndexSettings{
				Name:             "test_index_create",
				SearchableFields: []string{"Title", "content"}, // Use "Title" to match document field
				FilterableFields: []string{"category"},
				RankingCriteria: []config.RankingCriterion{
					{Field: "popularity", Order: "desc"},
				},
				MinWordSizeFor1Typo:  4,
				MinWordSizeFor2Typos: 7,
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing index name",
			requestBody: config.IndexSettings{
				SearchableFields: []string{"Title"},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/indexes", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestAddDocumentsHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// First create an index
	indexSettings := config.IndexSettings{
		Name:             "test_docs_add",
		SearchableFields: []string{"Title", "content"}, // Use "Title" to match document field
		FilterableFields: []string{"category"},
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name: "valid single document",
			requestBody: model.Document{
				"documentID": "test_doc_001",
				"Title":      "Test Document", // Use "Title" (capital T) as expected by the API
				"content":    "This is test content",
				"category":   "test",
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "valid multiple documents",
			requestBody: []model.Document{
				{
					"documentID": "test_doc_001",
					"Title":      "Doc 1", // Use "Title" (capital T) as expected by the API
					"content":    "Content 1",
					"category":   "test",
				},
				{
					"documentID": "test_doc_002",
					"Title":      "Doc 2", // Use "Title" (capital T) as expected by the API
					"content":    "Content 2",
					"category":   "test",
				},
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("PUT", "/indexes/test_docs_add/documents", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestSearchHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// Create index and add documents
	indexSettings := config.IndexSettings{
		Name:             "test_search_handler",
		SearchableFields: []string{"Title", "content"}, // Use "Title" to match document field
		FilterableFields: []string{"category"},
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	indexAccessor, _ := eng.GetIndex("test_search_handler")
	docs := []model.Document{
		{
			"documentID": "test_go_programming_doc",
			"Title":      "Go Programming", // Use "Title" (capital T) as expected by the API
			"content":    "Learn Go programming language",
			"category":   "programming",
		},
	}
	if err := indexAccessor.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    SearchRequest
		expectedStatus int
	}{
		{
			name: "valid search",
			requestBody: SearchRequest{
				Query:                    "Go programming",
				Page:                     1,
				PageSize:                 10,
				RestrictSearchableFields: []string{"Title", "content"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "search with filters",
			requestBody: SearchRequest{
				Query: "Go",
				Filters: map[string]interface{}{
					"category": "programming",
				},
				Page:                     1,
				PageSize:                 10,
				RestrictSearchableFields: []string{"Title", "content"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "success when RestrictSearchableFields not provided",
			requestBody: SearchRequest{
				Query:    "Go programming",
				Page:     1,
				PageSize: 10,
				// RestrictSearchableFields not provided - should use all configured searchable fields
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "error when RestrictSearchableFields contains invalid field",
			requestBody: SearchRequest{
				Query:                    "Go programming",
				Page:                     1,
				PageSize:                 10,
				RestrictSearchableFields: []string{"Title", "invalid_field"},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "search restricted to single field",
			requestBody: SearchRequest{
				Query:                    "Go programming",
				Page:                     1,
				PageSize:                 10,
				RestrictSearchableFields: []string{"Title"}, // Only search in Title
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/indexes/test_search_handler/_search", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestListIndexesHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	req, _ := http.NewRequest("GET", "/indexes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetIndexHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// Create an index first
	indexSettings := config.IndexSettings{
		Name:             "test_get_handler",
		SearchableFields: []string{"Title"},
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tests := []struct {
		name           string
		indexName      string
		expectedStatus int
	}{
		{
			name:           "existing index",
			indexName:      "test_get_handler",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existing index",
			indexName:      "non_existing",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/indexes/"+tt.indexName, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestDeleteIndexHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// First create an index
	indexSettings := config.IndexSettings{
		Name:             "test_delete",
		SearchableFields: []string{"Title"}, // Use "Title" to match document field
		FilterableFields: []string{"category"},
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tests := []struct {
		name           string
		indexName      string
		expectedStatus int
	}{
		{
			name:           "valid index deletion",
			indexName:      "test_delete",
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "non-existent index",
			indexName:      "non_existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/indexes/"+tt.indexName, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestUpdateIndexSettingsHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// Create test index with initial settings
	indexSettings := config.IndexSettings{
		Name:                      "test_update_settings",
		SearchableFields:          []string{"title", "content"},
		FilterableFields:          []string{"category", "year"},
		RankingCriteria:           []config.RankingCriterion{{Field: "popularity", Order: "desc"}},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
		NoTypoToleranceFields:     []string{},
		DistinctField:             "",
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Add some test documents to verify reindexing works
	indexAccessor, _ := eng.GetIndex("test_update_settings")
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "Test Document 1",
			"content":    "This is content for document 1",
			"category":   "test",
			"year":       2023,
			"popularity": 95.5,
		},
		{
			"documentID": "doc2",
			"title":      "Test Document 2",
			"content":    "This is content for document 2",
			"category":   "example",
			"year":       2024,
			"popularity": 87.3,
		},
	}
	if err := indexAccessor.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	tests := []struct {
		name              string
		requestBody       map[string]interface{}
		expectedStatus    int
		expectedReindexed *bool
		expectError       bool
		errorContains     string
	}{
		{
			name: "update field-level settings only (no reindexing)",
			requestBody: map[string]interface{}{
				"fields_without_prefix_search": []string{"content"},
				"no_typo_tolerance_fields":     []string{"category"},
				"distinct_field":               "title",
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{false}[0],
		},
		{
			name: "update searchable fields (triggers reindexing)",
			requestBody: map[string]interface{}{
				"searchable_fields": []string{"title", "content", "category"},
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "update filterable fields (triggers reindexing)",
			requestBody: map[string]interface{}{
				"filterable_fields": []string{"category", "year", "popularity"},
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "update ranking criteria (triggers reindexing)",
			requestBody: map[string]interface{}{
				"ranking_criteria": []map[string]interface{}{
					{"field": "year", "order": "desc"},
					{"field": "popularity", "order": "desc"},
				},
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "update typo settings (triggers reindexing)",
			requestBody: map[string]interface{}{
				"min_word_size_for_1_typo":  3,
				"min_word_size_for_2_typos": 6,
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "comprehensive update (mix of settings)",
			requestBody: map[string]interface{}{
				"searchable_fields":            []string{"title", "content"},
				"filterable_fields":            []string{"category", "year"},
				"ranking_criteria":             []map[string]interface{}{{"field": "popularity", "order": "desc"}},
				"fields_without_prefix_search": []string{},
				"no_typo_tolerance_fields":     []string{},
				"distinct_field":               "",
			},
			expectedStatus:    http.StatusAccepted,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "invalid field name (contains filter operator suffix)",
			requestBody: map[string]interface{}{
				"searchable_fields": []string{"field_exact", "field_gt"},
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			errorContains:  "Field name validation failed",
		},
		{
			name:           "empty request body",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			errorContains:  "No valid updatable fields provided",
		},
		{
			name: "non-existent index",
			requestBody: map[string]interface{}{
				"distinct_field": "title",
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use different index name for non-existent index test
			indexName := "test_update_settings"
			if tt.name == "non-existent index" {
				indexName = "non_existent_index"
			}

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("PATCH", "/indexes/"+indexName+"/settings", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectError {
				var errorResp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
					t.Errorf("Failed to unmarshal error response: %v", err)
					return
				}
				if errorResp["error"] == nil {
					t.Errorf("Expected error in response, but got none")
				}
				if tt.errorContains != "" {
					errorStr := fmt.Sprintf("%v", errorResp["error"])
					if !bytes.Contains([]byte(errorStr), []byte(tt.errorContains)) {
						t.Errorf("Expected error to contain '%s', but got: %s", tt.errorContains, errorStr)
					}
				}
			} else {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
					return
				}

				if response["message"] == nil {
					t.Errorf("Expected success message in response")
				}

				if tt.expectedReindexed != nil {
					// For async operations, check reindexing_required field instead
					reindexingRequired, exists := response["reindexing_required"].(bool)
					if !exists {
						t.Errorf("Expected 'reindexing_required' field in response")
					} else if reindexingRequired != *tt.expectedReindexed {
						t.Errorf("Expected reindexing_required=%v, got reindexing_required=%v", *tt.expectedReindexed, reindexingRequired)
					}
				}
			}
		})
	}
}

func TestRenameIndexHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// Create test indexes
	indexSettings1 := config.IndexSettings{
		Name:             "test_rename_source",
		SearchableFields: []string{"title", "content"},
		FilterableFields: []string{"category"},
		RankingCriteria: []config.RankingCriterion{
			{Field: "popularity", Order: "desc"},
		},
		MinWordSizeFor1Typo:  4,
		MinWordSizeFor2Typos: 7,
	}
	if err := eng.CreateIndex(indexSettings1); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	indexSettings2 := config.IndexSettings{
		Name:             "existing_target",
		SearchableFields: []string{"title"},
		FilterableFields: []string{"status"},
	}
	if err := eng.CreateIndex(indexSettings2); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Add some documents to the source index to verify data persistence
	indexAccessor, _ := eng.GetIndex("test_rename_source")
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "Test Document 1",
			"content":    "Content 1",
			"category":   "test",
		},
		{
			"documentID": "doc2",
			"title":      "Test Document 2",
			"content":    "Content 2",
			"category":   "test",
		},
	}
	if err := indexAccessor.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	tests := []struct {
		name           string
		indexName      string
		requestBody    RenameIndexRequest
		expectedStatus int
		expectedFields map[string]interface{}
	}{
		{
			name:      "successful rename",
			indexName: "test_rename_source",
			requestBody: RenameIndexRequest{
				NewName: "renamed_index",
			},
			expectedStatus: http.StatusAccepted,
			expectedFields: map[string]interface{}{
				"message":  "Index rename started: 'test_rename_source' -> 'renamed_index'",
				"old_name": "test_rename_source",
				"new_name": "renamed_index",
			},
		},
		{
			name:      "invalid JSON",
			indexName: "test_rename_source",
			requestBody: RenameIndexRequest{
				NewName: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "empty new name",
			indexName: "test_rename_source",
			requestBody: RenameIndexRequest{
				NewName: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "new name with whitespace",
			indexName: "test_rename_source",
			requestBody: RenameIndexRequest{
				NewName: " invalid_name ",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "source index not found",
			indexName: "nonexistent_index",
			requestBody: RenameIndexRequest{
				NewName: "new_name",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "target name already exists",
			indexName: "existing_target",
			requestBody: RenameIndexRequest{
				NewName: "renamed_index", // This should exist from the first test
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:      "same old and new name",
			indexName: "existing_target",
			requestBody: RenameIndexRequest{
				NewName: "existing_target",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/indexes/%s/rename", tt.indexName), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			// For successful renames, verify the response fields
			if tt.expectedStatus == http.StatusAccepted && tt.expectedFields != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
					return
				}

				for key, expectedValue := range tt.expectedFields {
					if actualValue, exists := response[key]; !exists {
						t.Errorf("Expected field %s not found in response", key)
					} else if actualValue != expectedValue {
						t.Errorf("Expected %s to be %v, got %v", key, expectedValue, actualValue)
					}
				}

				// Note: Since rename is now async, we can't immediately verify the index was renamed
				// The actual rename happens in the background via the job system
			}
		})
	}
}

func TestMultiSearchHandler(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	// Create index and add test documents
	indexSettings := config.IndexSettings{
		Name:             "test_multi_search",
		SearchableFields: []string{"title", "cast", "genres"},
		FilterableFields: []string{"year", "rating", "genres"},
	}
	if err := eng.CreateIndex(indexSettings); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	indexAccessor, _ := eng.GetIndex("test_multi_search")
	docs := []model.Document{
		{
			"documentID": "movie_matrix_1999",
			"title":      "The Matrix",
			"cast":       []string{"Keanu Reeves", "Laurence Fishburne"},
			"genres":     []string{"Action", "Sci-Fi"},
			"year":       1999,
			"rating":     8.7,
		},
		{
			"documentID": "movie_john_wick_2014",
			"title":      "John Wick",
			"cast":       []string{"Keanu Reeves", "Michael Nyqvist"},
			"genres":     []string{"Action", "Thriller"},
			"year":       2014,
			"rating":     7.4,
		},
		{
			"documentID": "movie_inception_2010",
			"title":      "Inception",
			"cast":       []string{"Leonardo DiCaprio", "Marion Cotillard"},
			"genres":     []string{"Action", "Sci-Fi", "Thriller"},
			"year":       2010,
			"rating":     8.8,
		},
	}
	if err := indexAccessor.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    MultiSearchRequest
		expectedStatus int
		validateFunc   func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "separate queries execution",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:                     "title_search",
						Query:                    "matrix",
						RestrictSearchableFields: []string{"title"},
					},
					{
						Name:                     "cast_search",
						Query:                    "keanu",
						RestrictSearchableFields: []string{"cast"},
					},
				},
				Page:     1,
				PageSize: 10,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				// Check that we have individual results
				results, ok := response["results"].(map[string]interface{})
				if !ok {
					t.Error("Expected 'results' field in response")
					return
				}

				// Should have results for both queries
				if _, exists := results["title_search"]; !exists {
					t.Error("Expected 'title_search' results")
				}
				if _, exists := results["cast_search"]; !exists {
					t.Error("Expected 'cast_search' results")
				}

				// Check total queries
				if totalQueries, ok := response["total_queries"].(float64); !ok || totalQueries != 2 {
					t.Errorf("Expected total_queries=2, got %v", response["total_queries"])
				}

				// Check processing time
				if processingTime, ok := response["processing_time_ms"].(float64); !ok || processingTime <= 0 {
					t.Errorf("Expected positive processing_time_ms, got %v", response["processing_time_ms"])
				}
			},
		},
		{
			name: "queries with filters",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:                     "action_movies",
						Query:                    "action",
						RestrictSearchableFields: []string{"genres"},
						Filters: map[string]interface{}{
							"rating_gte": 7.0,
						},
					},
					{
						Name:                     "sci_fi_movies",
						Query:                    "sci-fi",
						RestrictSearchableFields: []string{"genres"},
						Filters: map[string]interface{}{
							"year_gte": 2000,
						},
					},
				},
				Page:     1,
				PageSize: 10,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				// Should have individual results
				if _, exists := response["results"]; !exists {
					t.Error("Expected 'results' field in response")
				}
			},
		},
		{
			name: "query with field restrictions",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:                     "title_only",
						Query:                    "matrix",
						RestrictSearchableFields: []string{"title"},
						RetrivableFields:         []string{"title", "year"},
					},
					{
						Name:                     "cast_only",
						Query:                    "keanu",
						RestrictSearchableFields: []string{"cast"},
						RetrivableFields:         []string{"title", "cast"},
					},
				},
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				// Basic validation that response structure is correct
				if _, exists := response["results"]; !exists {
					t.Error("Expected 'results' field in response")
				}
			},
		},
		{
			name: "query with typo tolerance overrides",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:                     "exact_search",
						Query:                    "matrix",
						RestrictSearchableFields: []string{"title"},
						MinWordSizeFor1Typo:      &[]int{0}[0], // Disable typo tolerance
						MinWordSizeFor2Typos:     &[]int{0}[0],
					},
					{
						Name:                     "fuzzy_search",
						Query:                    "matric", // Typo
						RestrictSearchableFields: []string{"title"},
						MinWordSizeFor1Typo:      &[]int{3}[0], // Enable typo tolerance
						MinWordSizeFor2Typos:     &[]int{6}[0],
					},
				},
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				// Should have individual results for both queries
				results, ok := response["results"].(map[string]interface{})
				if !ok {
					t.Error("Expected 'results' field in response")
					return
				}

				if _, exists := results["exact_search"]; !exists {
					t.Error("Expected 'exact_search' results")
				}
				if _, exists := results["fuzzy_search"]; !exists {
					t.Error("Expected 'fuzzy_search' results")
				}
			},
		},
		{
			name: "empty queries array",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{},
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				if errorMsg, exists := response["error"]; !exists {
					t.Error("Expected error message for empty queries")
				} else if !bytes.Contains([]byte(fmt.Sprintf("%v", errorMsg)), []byte("At least one query is required")) {
					t.Errorf("Expected error about empty queries, got: %v", errorMsg)
				}
			},
		},
		{
			name: "duplicate query names",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:  "duplicate_name",
						Query: "matrix",
					},
					{
						Name:  "duplicate_name",
						Query: "inception",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				if errorMsg, exists := response["error"]; !exists {
					t.Error("Expected error message for duplicate query names")
				} else if !bytes.Contains([]byte(fmt.Sprintf("%v", errorMsg)), []byte("Query names must be unique")) {
					t.Errorf("Expected error about duplicate names, got: %v", errorMsg)
				}
			},
		},
		{
			name: "empty query name",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:  "",
						Query: "matrix",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				if errorMsg, exists := response["error"]; !exists {
					t.Error("Expected error message for empty query name")
				} else if !bytes.Contains([]byte(fmt.Sprintf("%v", errorMsg)), []byte("non-empty name")) {
					t.Errorf("Expected error about empty query name, got: %v", errorMsg)
				}
			},
		},
		{
			name: "invalid restrict_searchable_fields",
			requestBody: MultiSearchRequest{
				Queries: []NamedSearchRequest{
					{
						Name:                     "invalid_field_search",
						Query:                    "matrix",
						RestrictSearchableFields: []string{"invalid_field"},
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, response map[string]interface{}) {
				if errorMsg, exists := response["error"]; !exists {
					t.Error("Expected error message for invalid searchable field")
				} else if !bytes.Contains([]byte(fmt.Sprintf("%v", errorMsg)), []byte("not configured as a searchable field")) {
					t.Errorf("Expected error about invalid searchable field, got: %v", errorMsg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/indexes/test_multi_search/_multi_search", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, response)
			}
		})
	}
}

func TestMultiSearchHandler_IndexNotFound(t *testing.T) {
	eng := setupTestEngine()
	router := setupTestRouter(eng)

	requestBody := MultiSearchRequest{
		Queries: []NamedSearchRequest{
			{
				Name:  "test_query",
				Query: "test",
			},
		},
	}

	body, _ := json.Marshal(requestBody)
	req, _ := http.NewRequest("POST", "/indexes/nonexistent_index/_multi_search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
		return
	}

	if errorMsg, exists := response["error"]; !exists {
		t.Error("Expected error message for nonexistent index")
	} else if !bytes.Contains([]byte(fmt.Sprintf("%v", errorMsg)), []byte("not found")) {
		t.Errorf("Expected error about index not found, got: %v", errorMsg)
	}
}

func TestMain(m *testing.M) {
	// Setup code before tests
	code := m.Run()
	// Cleanup code after tests
	// Remove all registered test directories
	testDirsMu.Lock()
	for _, testDir := range testDirs {
		if err := os.RemoveAll(testDir); err != nil {
			fmt.Printf("Warning: Failed to remove test directory %s: %v\n", testDir, err)
		}
	}
	testDirsMu.Unlock()
	os.Exit(code)
}

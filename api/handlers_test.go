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
	"github.com/gcbaptista/go-search-engine/services"
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
			expectedStatus: http.StatusCreated,
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
	eng.CreateIndex(indexSettings)

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
			expectedStatus: http.StatusOK,
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
			expectedStatus: http.StatusOK,
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
	eng.CreateIndex(indexSettings)

	indexAccessor, _ := eng.GetIndex("test_search_handler")
	docs := []model.Document{
		{
			"documentID": "test_go_programming_doc",
			"Title":      "Go Programming", // Use "Title" (capital T) as expected by the API
			"content":    "Learn Go programming language",
			"category":   "programming",
		},
	}
	indexAccessor.AddDocuments(docs)

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
	eng.CreateIndex(indexSettings)

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
	eng.CreateIndex(indexSettings)

	tests := []struct {
		name           string
		indexName      string
		expectedStatus int
	}{
		{
			name:           "valid index deletion",
			indexName:      "test_delete",
			expectedStatus: http.StatusOK,
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
	eng.CreateIndex(indexSettings)

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
	indexAccessor.AddDocuments(docs)

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
			expectedStatus:    http.StatusOK,
			expectedReindexed: &[]bool{false}[0],
		},
		{
			name: "update searchable fields (triggers reindexing)",
			requestBody: map[string]interface{}{
				"searchable_fields": []string{"title", "content", "category"},
			},
			expectedStatus:    http.StatusOK,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "update filterable fields (triggers reindexing)",
			requestBody: map[string]interface{}{
				"filterable_fields": []string{"category", "year", "popularity"},
			},
			expectedStatus:    http.StatusOK,
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
			expectedStatus:    http.StatusOK,
			expectedReindexed: &[]bool{true}[0],
		},
		{
			name: "update typo settings (triggers reindexing)",
			requestBody: map[string]interface{}{
				"min_word_size_for_1_typo":  3,
				"min_word_size_for_2_typos": 6,
			},
			expectedStatus:    http.StatusOK,
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
			expectedStatus:    http.StatusOK,
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
				json.Unmarshal(w.Body.Bytes(), &errorResp)
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
				json.Unmarshal(w.Body.Bytes(), &response)

				if response["message"] == nil {
					t.Errorf("Expected success message in response")
				}

				if tt.expectedReindexed != nil {
					reindexed, exists := response["reindexed"].(bool)
					if !exists {
						t.Errorf("Expected 'reindexed' field in response")
					} else if reindexed != *tt.expectedReindexed {
						t.Errorf("Expected reindexed=%v, got reindexed=%v", *tt.expectedReindexed, reindexed)
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
	eng.CreateIndex(indexSettings1)

	indexSettings2 := config.IndexSettings{
		Name:             "existing_target",
		SearchableFields: []string{"title"},
		FilterableFields: []string{"status"},
	}
	eng.CreateIndex(indexSettings2)

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
	indexAccessor.AddDocuments(docs)

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
			expectedStatus: http.StatusOK,
			expectedFields: map[string]interface{}{
				"message":  "Index renamed successfully",
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

			// For successful renames, verify the response fields and data persistence
			if tt.expectedStatus == http.StatusOK && tt.expectedFields != nil {
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

				// Verify the index was actually renamed
				newName := tt.requestBody.NewName
				renamedIndex, err := eng.GetIndex(newName)
				if err != nil {
					t.Errorf("Expected to find renamed index '%s', but got error: %v", newName, err)
				} else {
					// Verify settings were updated
					settings := renamedIndex.Settings()
					if settings.Name != newName {
						t.Errorf("Expected renamed index to have name '%s', got '%s'", newName, settings.Name)
					}

					// Verify documents are still accessible
					searchResult, err := renamedIndex.Search(services.SearchQuery{
						QueryString: "Test Document",
						Page:        1,
						PageSize:    10,
					})
					if err != nil {
						t.Errorf("Failed to search in renamed index: %v", err)
					} else if len(searchResult.Hits) != 2 {
						t.Errorf("Expected 2 documents in renamed index, got %d", len(searchResult.Hits))
					}
				}

				// Verify old index no longer exists
				_, err = eng.GetIndex(tt.indexName)
				if err == nil {
					t.Errorf("Expected old index '%s' to be removed, but it still exists", tt.indexName)
				}
			}
		})
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

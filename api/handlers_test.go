package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gin-gonic/gin"
)

func setupTestEngine() *engine.Engine {
	// Use unique test directory for each test run
	testDir := fmt.Sprintf("./test_data_%d", time.Now().UnixNano())
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
				Query:    "Go programming",
				Page:     1,
				PageSize: 10,
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
				Page:     1,
				PageSize: 10,
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

	// Create an index first
	indexSettings := config.IndexSettings{
		Name:             "test_delete_handler",
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
			indexName:      "test_delete_handler",
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
			req, _ := http.NewRequest("DELETE", "/indexes/"+tt.indexName, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Cleanup function to remove test directories
func TestMain(m *testing.M) {
	code := m.Run()

	// Clean up test directories
	if entries, err := os.ReadDir("."); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && len(entry.Name()) > 10 && entry.Name()[:10] == "test_data_" {
				os.RemoveAll(entry.Name())
			}
		}
	}

	os.Exit(code)
}

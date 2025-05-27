package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gcbaptista/go-search-engine/store"
	"github.com/stretchr/testify/assert"
)

// --- Test Helpers ---

func newTestIndexSettings() *config.IndexSettings {
	return &config.IndexSettings{
		Name:                      "test_search_index",
		SearchableFields:          []string{"title", "description", "tags"},
		FilterableFields:          []string{"genre", "year", "rating", "is_available", "release_date", "features"},
		RankingCriteria:           []config.RankingCriterion{{Field: "~score", Order: "desc"}, {Field: "popularity", Order: "desc"}},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{}, // N-grams enabled for all searchable fields by default for search tests
	}
}

// setupTestSearchService creates a new search service with an indexing service
// to easily add documents for testing search functionality.
func setupTestSearchService(t *testing.T, settings *config.IndexSettings) (*Service, *indexing.Service) {
	t.Helper()
	if settings == nil {
		settings = newTestIndexSettings()
	}

	invIdx := &index.InvertedIndex{
		Index:    make(map[string]index.PostingList),
		Settings: settings,
	}
	docStore := &store.DocumentStore{
		Docs:                   make(map[uint32]model.Document),
		ExternalIDtoInternalID: make(map[string]uint32),
		NextID:                 0,
	}

	indexerService, err := indexing.NewService(invIdx, docStore)
	if err != nil {
		t.Fatalf("Failed to create indexing service: %v", err)
	}

	searchService, err := NewService(invIdx, docStore, settings)
	if err != nil {
		t.Fatalf("Failed to create search service: %v", err)
	}
	return searchService, indexerService
}

// --- Test Cases ---

func TestNewService(t *testing.T) {
	t.Run("valid initialization", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestIndexSettings()}
		docStore := &store.DocumentStore{}
		settings := newTestIndexSettings()
		_, err := NewService(invIdx, docStore, settings)
		if err != nil {
			t.Errorf("NewService() error = %v, wantErr nil", err)
		}
	})

	t.Run("nil inverted index", func(t *testing.T) {
		docStore := &store.DocumentStore{}
		settings := newTestIndexSettings()
		_, err := NewService(nil, docStore, settings)
		if err == nil {
			t.Error("NewService() with nil invertedIndex, wantErr, got nil")
		}
	})

	t.Run("nil document store", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestIndexSettings()}
		settings := newTestIndexSettings()
		_, err := NewService(invIdx, nil, settings)
		if err == nil {
			t.Error("NewService() with nil documentStore, wantErr, got nil")
		}
	})

	t.Run("nil settings", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestIndexSettings()}
		docStore := &store.DocumentStore{}
		_, err := NewService(invIdx, docStore, nil)
		if err == nil {
			t.Error("NewService() with nil settings, wantErr, got nil")
		}
	})
}

// TestSearch_TermLogic focuses on term matching (exact, typo), intersection, and candidate hit generation.
// It does not test final sorting or pagination as that logic is not fully present in the current service.go.
func TestSearch_TermLogic(t *testing.T) {
	docID1 := "test_hello_world_doc"
	docID2 := "test_hallo_welt_doc"
	docID3 := "test_another_example_doc"

	doc1 := model.Document{"documentID": docID1, "title": "Hello World", "description": "A simple program about the world.", "tags": []string{"greeting", "example"}, "popularity": 10.0}
	doc2 := model.Document{"documentID": docID2, "title": "Hallo Welt", "description": "Ein einfaches Program.", "tags": []string{"greeting", "german"}, "popularity": 8.0}
	doc3 := model.Document{"documentID": docID3, "title": "Another Example", "description": "Just another world example.", "tags": []string{"example", "random"}, "popularity": 9.0}

	service, indexer := setupTestSearchService(t, nil) // Uses default settings (ngrams on for all fields)

	if err := indexer.AddDocuments([]model.Document{doc1, doc2, doc3}); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	t.Run("single exact term match", func(t *testing.T) {
		query := services.SearchQuery{QueryString: "Hello", RestrictSearchableFields: []string{"title", "description", "tags"}}
		// We can't fully test SearchResult, so we'll inspect internal state or a limited SearchResult.
		// For now, assume Search populates internal candidateHits map before returning.
		// This part of the test needs to be adapted once Search is fully implemented.
		// Let's assume for now we are checking the intersectedDocIDs or some precursor.

		// Temporarily, to allow compilation, we'll call Search and expect no error.
		// The actual assertions will need to be more specific to candidateHit structures later.
		_, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// Expected: doc1 should be among candidates for "hello".
	})

	t.Run("multiple terms AND logic", func(t *testing.T) {
		query := services.SearchQuery{QueryString: "world example", RestrictSearchableFields: []string{"title", "description", "tags"}}
		_, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}
		// Expected: doc1 and doc3 (both contain "world" and "example" in some form across fields)
	})

	t.Run("typo match (1 typo)", func(t *testing.T) {
		// "Hallo" vs "Hllo" (indexed) -> Levenshtein = 1
		// Need to ensure "Hllo" is indexed or use an existing term.
		// Let's use "Helo" for "Hello" (from doc1)
		query := services.SearchQuery{QueryString: "Helo", RestrictSearchableFields: []string{"title", "description", "tags"}}
		_, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}
		// Expected: doc1 (due to typo for "hello")
	})

	t.Run("typo match (2 typos)", func(t *testing.T) {
		// Let's assume settings allow 2 typos for longer words.
		// queryToken "prograam" for "program"
		query := services.SearchQuery{QueryString: "prograam", RestrictSearchableFields: []string{"title", "description", "tags"}} // from doc1 description ("program")
		_, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}
		// Expected: doc1, doc2 (doc1 has "program", doc2 has "program" - assuming tokenizer makes it so)
	})

	t.Run("no match", func(t *testing.T) {
		query := services.SearchQuery{QueryString: "nonexistentXYZ", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}
		if len(result.Hits) != 0 || result.Total != 0 {
			t.Errorf("Expected 0 hits for no match, got %d hits, total %d", len(result.Hits), result.Total)
		}
	})

	t.Run("empty query string", func(t *testing.T) {
		query := services.SearchQuery{QueryString: "", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}
		if len(result.Hits) != 0 || result.Total != 0 {
			t.Errorf("Expected 0 hits for empty query, got %d hits, total %d", len(result.Hits), result.Total)
		}
	})

	t.Run("took field returns milliseconds", func(t *testing.T) {
		query := services.SearchQuery{QueryString: "Hello", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// Verify that Took is a reasonable milliseconds value (should be >= 0 and likely < 1000ms for a simple search)
		// Note: Very fast operations may legitimately take < 1ms and round down to 0
		if result.Took < 0 {
			t.Errorf("Expected Took to be non-negative milliseconds, got %d", result.Took)
		}
		if result.Took > 10000 { // 10 seconds would be unreasonably long for a test search
			t.Errorf("Expected Took to be reasonable for a test search (< 10s), got %d ms", result.Took)
		}
	})

	t.Run("took field demonstrates milliseconds conversion", func(t *testing.T) {
		// This test verifies that the time.Since().Milliseconds() conversion is working
		// We can't easily control timing, but we can verify the field is populated
		query := services.SearchQuery{QueryString: "Hello", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result, err := service.Search(query)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// The Took field should be populated (even if 0 for very fast operations)
		// This is more of a "does not panic" test than a specific value test
		_ = result.Took // Just ensure it's accessible
	})

	t.Run("query returns unique query ID", func(t *testing.T) {
		query1 := services.SearchQuery{QueryString: "Hello", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result1, err := service.Search(query1)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		query2 := services.SearchQuery{QueryString: "World", RestrictSearchableFields: []string{"title", "description", "tags"}}
		result2, err := service.Search(query2)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// Verify QueryId is populated
		if result1.QueryId == "" {
			t.Errorf("Expected QueryId to be non-empty, got empty string")
		}

		// Verify QueryId looks like a UUID (36 characters with hyphens)
		if len(result1.QueryId) != 36 {
			t.Errorf("Expected QueryId to be 36 characters long (UUID format), got %d characters", len(result1.QueryId))
		}

		// Verify different queries get different IDs
		if result1.QueryId == result2.QueryId {
			t.Errorf("Expected different QueryIds for different queries, both got %s", result1.QueryId)
		}
	})

	t.Run("dynamic typo limit calculation", func(t *testing.T) {
		// Test the dynamic limit logic by checking the algorithm used in the search
		// We can't directly test the internal logic, but we can verify it works with different page sizes

		// Small page size - should use base limit (500)
		smallQuery := services.SearchQuery{QueryString: "Hello", PageSize: 10, RestrictSearchableFields: []string{"title", "description", "tags"}}
		_, err := service.Search(smallQuery)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// Large page size - should use dynamic limit (pageSize * 10)
		largeQuery := services.SearchQuery{QueryString: "Hello", PageSize: 100, RestrictSearchableFields: []string{"title", "description", "tags"}}
		_, err = service.Search(largeQuery)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// Very large page size - should be capped at 2000
		veryLargeQuery := services.SearchQuery{QueryString: "Hello", PageSize: 500, RestrictSearchableFields: []string{"title", "description", "tags"}}
		_, err = service.Search(veryLargeQuery)
		if err != nil {
			t.Errorf("Search() error = %v", err)
		}

		// All searches should work without error, demonstrating the dynamic limit scales appropriately
	})

	// More tests to be added for:
	// - Score aggregation in candidateHit
	// - matchedQueryTermsByField population in candidateHit
	// - Interaction of exact and typo matches for the same document and token
}

func TestDeduplicateResults(t *testing.T) {
	service, _ := setupTestSearchService(t, nil)

	// Create test hits with duplicate titles
	hits := []services.HitResult{
		{
			Document: model.Document{"documentID": "1", "title": "The Matrix", "year": 1999, "rating": 8.7},
			Score:    10.0,
		},
		{
			Document: model.Document{"documentID": "2", "title": "The Matrix", "year": 1999, "rating": 8.7},
			Score:    9.0,
		},
		{
			Document: model.Document{"documentID": "3", "title": "The Dark Knight", "year": 2008, "rating": 9.0},
			Score:    8.0,
		},
		{
			Document: model.Document{"documentID": "4", "title": "The Dark Knight", "year": 2008, "rating": 9.0},
			Score:    7.0,
		},
		{
			Document: model.Document{"documentID": "5", "title": "Inception", "year": 2010, "rating": 8.8},
			Score:    6.0,
		},
	}

	t.Run("no deduplication when distinct field is empty", func(t *testing.T) {
		result := service.deduplicateResults(hits, "")
		if len(result) != len(hits) {
			t.Errorf("Expected %d hits, got %d", len(hits), len(result))
		}
	})

	t.Run("deduplication by title keeps highest scoring", func(t *testing.T) {
		result := service.deduplicateResults(hits, "title")

		// Should have 3 unique titles: The Matrix, The Dark Knight, Inception
		if len(result) != 3 {
			t.Errorf("Expected 3 deduplicated hits, got %d", len(result))
		}

		// Verify the kept documents are the highest scoring ones
		expectedUUIDs := []string{"1", "3", "5"} // These have the highest scores for each title
		for i, hit := range result {
			if hit.Document["documentID"] != expectedUUIDs[i] {
				t.Errorf("Expected documentID %s at position %d, got %s", expectedUUIDs[i], i, hit.Document["documentID"])
			}
		}
	})

	t.Run("deduplication handles missing field", func(t *testing.T) {
		hitsWithMissingField := []services.HitResult{
			{
				Document: model.Document{"documentID": "1", "title": "The Matrix"},
				Score:    10.0,
			},
			{
				Document: model.Document{"documentID": "2", "genre": "Action"}, // Missing title
				Score:    9.0,
			},
		}

		result := service.deduplicateResults(hitsWithMissingField, "title")

		// Both should be kept since one doesn't have the distinct field
		if len(result) != 2 {
			t.Errorf("Expected 2 hits (one with missing field), got %d", len(result))
		}
	})

	t.Run("deduplication by year", func(t *testing.T) {
		result := service.deduplicateResults(hits, "year")

		// Should have 3 unique years: 1999, 2008, 2010
		if len(result) != 3 {
			t.Errorf("Expected 3 deduplicated hits by year, got %d", len(result))
		}
	})
}

// TestApplyFilterLogic needs to be comprehensive for types and operators
func TestApplyFilterLogic(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		docValue    interface{}
		operator    string
		filterValue interface{}
		expected    bool
	}{
		// String comparisons
		{"string exact pass", "hello", "", "hello", true},
		{"string exact fail", "hello", "", "world", false},
		{"string _ne pass", "hello", "_ne", "world", true},
		{"string _ne fail", "hello", "_ne", "hello", false},
		{"string _contains pass", "hello world", "_contains", "world", true},
		{"string _contains fail", "hello world", "_contains", "xyz", false},
		{"string _ncontains pass", "hello world", "_ncontains", "xyz", true},
		{"string _ncontains fail", "hello world", "_ncontains", "world", false},

		// Float64 comparisons (doc value is float64)
		{"float exact pass", 10.5, "", 10.5, true},
		{"float exact fail int filter", 10.5, "", 10, false}, // Filter value type matters
		{"float _ne pass", 10.5, "_ne", 10.6, true},
		{"float _gte pass", 10.5, "_gte", 10.5, true},
		{"float _gte also pass", 10.5, "_gte", 10.4, true},
		{"float _gt pass", 10.5, "_gt", 10.4, true},
		{"float _gt fail", 10.5, "_gt", 10.5, false},
		{"float _lte pass", 10.5, "_lte", 10.5, true},
		{"float _lte also pass", 10.5, "_lte", 10.6, true},
		{"float _lt pass", 10.5, "_lt", 10.6, true},
		{"float _lt fail", 10.5, "_lt", 10.5, false},

		// Integer comparisons (doc value is int, filter value can be int or float)
		{"int exact pass (int filter)", 10, "", 10, true},
		{"int exact pass (float filter)", 10, "", 10.0, true},
		{"int exact fail", 10, "", 11, false},
		{"int _gte pass", 10, "_gte", 10.0, true},
		{"int _gt fail", 10, "_gt", 10, false},

		// Bool comparisons
		{"bool exact pass", true, "", true, true},
		{"bool exact fail", true, "", false, false},
		{"bool _ne pass", true, "_ne", false, true},

		// Time comparisons (doc value is time.Time)
		{"time exact pass", now, "", now.Format(time.RFC3339Nano), true},
		{"time exact fail", now, "", now.Add(time.Second).Format(time.RFC3339Nano), false},
		{"time _gte pass", now, "_gte", now.Format(time.RFC3339Nano), true},
		{"time _gt fail", now, "_gt", now.Format(time.RFC3339Nano), false},
		{"time _lt pass", now, "_lt", now.Add(time.Second).Format(time.RFC3339Nano), true},

		// Slice of strings comparisons (doc value is []string)
		{"[]string _contains_any_of pass", []string{"a", "b"}, "_contains_any_of", []interface{}{"b", "c"}, true},
		{"[]string _contains_any_of fail", []string{"a", "b"}, "_contains_any_of", []interface{}{"c", "d"}, false},
		{"[]string _contains pass (single string filter)", []string{"a", "b"}, "_contains", "a", true},
		{"[]string _contains fail (single string filter)", []string{"a", "b"}, "_contains", "c", false},

		// Slice of interface{} (containing strings) comparisons (doc value is []interface{})
		{"[]interface{} _contains_any_of pass", []interface{}{"x", "y"}, "_contains_any_of", []interface{}{"y", "z"}, true},
		{"[]interface{} _contains pass (single string filter)", []interface{}{"x", "y"}, "_contains", "x", true},

		// Invalid filter value type for operator
		{"string exact with int filter", "hello", "", 123, false},
		{"float exact with string filter (should pass with conversion)", 10.5, "", "10.5", true}, // String to float conversion should work
		{"float exact with non-numeric string filter", 10.5, "", "not-a-number", false},          // Non-numeric string should fail
		{"time exact with int filter", now, "", 12345, false},

		// Test string to number conversions
		{"int exact with string number", 2011, "", "2011", true}, // This is the main use case we're fixing
		{"float _gte with string number", 10.5, "_gte", "10.0", true},
		{"float _lt with string number", 5.0, "_lt", "10.5", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// applyFilterLogic(docFieldVal interface{}, operator string, filterValue interface{}, fieldNameForDebug, indexNameForDebug string)
			// For fieldNameForDebug and indexNameForDebug, we can use dummy values as they are for logging.
			got := applyFilterLogic(tc.docValue, tc.operator, tc.filterValue, "testField", "testIndex")
			if got != tc.expected {
				t.Errorf("applyFilterLogic(%v, %q, %v) = %v, want %v", tc.docValue, tc.operator, tc.filterValue, got, tc.expected)
			}
		})
	}
}

func TestSearchWithDeduplication(t *testing.T) {
	// Create settings with deduplication enabled
	settings := newTestIndexSettings()
	settings.DistinctField = "title" // Enable deduplication by title

	service, indexer := setupTestSearchService(t, settings)

	// Add multiple documents with duplicate titles
	uuid1 := "test_matrix_doc1"
	uuid2 := "test_matrix_doc2"
	uuid3 := "test_dark_knight_doc1"
	uuid4 := "test_dark_knight_doc2"
	uuid5 := "test_inception_doc"
	uuid6 := "test_pulp_fiction_doc"

	docs := []model.Document{
		{"documentID": uuid1, "title": "The Matrix", "year": 1999, "rating": 8.7, "popularity": 92.0},
		{"documentID": uuid2, "title": "The Matrix", "year": 1999, "rating": 8.7, "popularity": 91.0}, // Duplicate with lower popularity
		{"documentID": uuid3, "title": "The Dark Knight", "year": 2008, "rating": 9.0, "popularity": 96.0},
		{"documentID": uuid4, "title": "The Dark Knight", "year": 2008, "rating": 9.0, "popularity": 95.0}, // Duplicate with lower popularity
		{"documentID": uuid5, "title": "Inception", "year": 2010, "rating": 8.8, "popularity": 87.0},
		{"documentID": uuid6, "title": "Pulp Fiction", "year": 1994, "rating": 8.9, "popularity": 94.0},
	}

	if err := indexer.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	t.Run("search with deduplication returns unique titles", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "the", // Should match both Matrix documents and both Dark Knight documents
			RestrictSearchableFields: []string{"title", "description", "tags"},
		}

		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should return only 2 unique documents (highest scoring for each title)
		if len(result.Hits) != 2 {
			t.Errorf("Expected 2 unique hits, got %d", len(result.Hits))
		}

		// Verify we got the right documents (highest popularity ones)
		foundTitles := make(map[string]string) // title -> uuid
		for _, hit := range result.Hits {
			title := hit.Document["title"].(string)
			docUUID := hit.Document["documentID"].(string)
			foundTitles[title] = docUUID
		}

		// Should have kept the highest popularity versions
		if foundTitles["The Matrix"] != uuid1 {
			t.Errorf("Expected to keep The Matrix with documentID %s (highest popularity), got documentID %s", uuid1, foundTitles["The Matrix"])
		}
		if foundTitles["The Dark Knight"] != uuid3 {
			t.Errorf("Expected to keep The Dark Knight with documentID %s (highest popularity), got documentID %s", uuid3, foundTitles["The Dark Knight"])
		}
	})

	t.Run("search without deduplication field returns all matches", func(t *testing.T) {
		// Create a service without deduplication
		settingsNoDedup := newTestIndexSettings()
		settingsNoDedup.DistinctField = "" // No deduplication

		serviceNoDedup, indexerNoDedup := setupTestSearchService(t, settingsNoDedup)
		if err := indexerNoDedup.AddDocuments(docs); err != nil {
			t.Fatalf("Failed to add documents: %v", err)
		}

		query := services.SearchQuery{
			QueryString:              "the", // Should match both Matrix documents and both Dark Knight documents
			RestrictSearchableFields: []string{"title", "description", "tags"},
		}

		result, err := serviceNoDedup.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should return 4 documents (both duplicates for each title)
		if len(result.Hits) != 4 {
			t.Errorf("Expected 4 hits without deduplication, got %d", len(result.Hits))
		}
	})
}

func TestFieldNameValidation(t *testing.T) {
	t.Run("validate field names with operator conflicts", func(t *testing.T) {
		settings := &config.IndexSettings{
			Name:             "test_index",
			SearchableFields: []string{"title", "description_contains", "user_exact"}, // These have conflicts
			FilterableFields: []string{"year", "rating_gte", "status_ne"},             // These have conflicts
			DistinctField:    "uuid_exact",                                            // This has a conflict
		}

		conflicts := settings.ValidateFieldNames()

		expectedConflicts := []string{
			"Field 'description_contains' ends with operator '_contains' which may cause parsing conflicts",
			"Field 'user_exact' ends with operator '_exact' which may cause parsing conflicts",
			"Field 'rating_gte' ends with operator '_gte' which may cause parsing conflicts",
			"Field 'status_ne' ends with operator '_ne' which may cause parsing conflicts",
			"Field 'uuid_exact' ends with operator '_exact' which may cause parsing conflicts",
		}

		if len(conflicts) != len(expectedConflicts) {
			t.Errorf("Expected %d conflicts, got %d: %v", len(expectedConflicts), len(conflicts), conflicts)
		}

		for i, expected := range expectedConflicts {
			if i < len(conflicts) && conflicts[i] != expected {
				t.Errorf("Expected conflict %d to be %q, got %q", i, expected, conflicts[i])
			}
		}
	})

	t.Run("validate field names without conflicts", func(t *testing.T) {
		settings := &config.IndexSettings{
			Name:             "test_index",
			SearchableFields: []string{"title", "description", "content"},
			FilterableFields: []string{"year", "rating", "popularity", "release_date"},
			DistinctField:    "uuid",
		}

		conflicts := settings.ValidateFieldNames()

		if len(conflicts) != 0 {
			t.Errorf("Expected no conflicts, got %d: %v", len(conflicts), conflicts)
		}
	})

}

func TestRestrictSearchableFields(t *testing.T) {
	service, indexer := setupTestSearchService(t, nil)

	// Add test documents with content in different fields
	docs := []model.Document{
		{
			"documentID":  "doc1",
			"title":       "Hello World",
			"description": "A simple program",
			"tags":        []string{"greeting", "example"},
		},
		{
			"documentID":  "doc2",
			"title":       "Programming Guide",
			"description": "Hello developers",
			"tags":        []string{"guide", "tutorial"},
		},
	}

	if err := indexer.AddDocuments(docs); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	t.Run("success when RestrictSearchableFields is not provided - uses all configured fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString: "Hello",
			// RestrictSearchableFields not provided - should use all configured searchable fields
		}

		result, err := service.Search(query)
		if err != nil {
			t.Errorf("Expected success when RestrictSearchableFields is not provided, got error: %v", err)
		}

		// Should find both documents since "Hello" appears in both title and description
		if len(result.Hits) != 2 {
			t.Errorf("Expected 2 hits when using all configured fields, got %d", len(result.Hits))
		}
	})

	t.Run("error when RestrictSearchableFields contains invalid field", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Hello",
			RestrictSearchableFields: []string{"title", "invalid_field"},
		}

		_, err := service.Search(query)
		if err == nil {
			t.Error("Expected error when RestrictSearchableFields contains invalid field, got nil")
		}
		if !strings.Contains(err.Error(), "not configured as a searchable field") {
			t.Errorf("Expected error about invalid field, got: %v", err)
		}
	})

	t.Run("search restricted to title field only", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Hello",
			RestrictSearchableFields: []string{"title"}, // Only search in title
		}

		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find doc1 (has "Hello" in title) but not doc2 (has "Hello" in description)
		if len(result.Hits) != 1 {
			t.Errorf("Expected 1 hit when searching only in title, got %d", len(result.Hits))
		}

		if len(result.Hits) > 0 {
			foundDoc := result.Hits[0].Document
			if foundDoc["documentID"] != "doc1" {
				t.Errorf("Expected to find doc1, got %s", foundDoc["documentID"])
			}

			// Verify field matches only include title
			fieldMatches := result.Hits[0].FieldMatches
			if _, hasTitle := fieldMatches["title"]; !hasTitle {
				t.Error("Expected field matches to include title")
			}
			if _, hasDescription := fieldMatches["description"]; hasDescription {
				t.Error("Expected field matches to NOT include description when restricted to title only")
			}
		}
	})

	t.Run("search restricted to description field only", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Hello",
			RestrictSearchableFields: []string{"description"}, // Only search in description
		}

		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find doc2 (has "Hello" in description) but not doc1 (has "Hello" in title)
		if len(result.Hits) != 1 {
			t.Errorf("Expected 1 hit when searching only in description, got %d", len(result.Hits))
		}

		if len(result.Hits) > 0 {
			foundDoc := result.Hits[0].Document
			if foundDoc["documentID"] != "doc2" {
				t.Errorf("Expected to find doc2, got %s", foundDoc["documentID"])
			}

			// Verify field matches only include description
			fieldMatches := result.Hits[0].FieldMatches
			if _, hasDescription := fieldMatches["description"]; !hasDescription {
				t.Error("Expected field matches to include description")
			}
			if _, hasTitle := fieldMatches["title"]; hasTitle {
				t.Error("Expected field matches to NOT include title when restricted to description only")
			}
		}
	})

	t.Run("search restricted to multiple fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Hello",
			RestrictSearchableFields: []string{"title", "description"}, // Search in both title and description
		}

		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find both documents
		if len(result.Hits) != 2 {
			t.Errorf("Expected 2 hits when searching in title and description, got %d", len(result.Hits))
		}
	})

	t.Run("search with all configured searchable fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Hello",
			RestrictSearchableFields: []string{"title", "description", "tags"}, // All configured fields
		}

		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find both documents (same as searching in title and description since "Hello" is not in tags)
		if len(result.Hits) != 2 {
			t.Errorf("Expected 2 hits when searching in all fields, got %d", len(result.Hits))
		}
	})
}

// TestRetrievableFields tests the functionality of limiting returned document fields
func TestRetrievableFields(t *testing.T) {
	docID1 := "test_movie_1"
	docID2 := "test_movie_2"

	doc1 := model.Document{
		"documentID":  docID1,
		"title":       "The Matrix",
		"description": "A computer hacker learns about the true nature of reality",
		"year":        1999,
		"rating":      8.7,
		"director":    "The Wachowskis",
		"genre":       "Sci-Fi",
	}
	doc2 := model.Document{
		"documentID":  docID2,
		"title":       "The Matrix Reloaded",
		"description": "Neo and the rebel leaders estimate that they have 72 hours",
		"year":        2003,
		"rating":      7.2,
		"director":    "The Wachowskis",
		"genre":       "Sci-Fi",
	}

	service, indexer := setupTestSearchService(t, nil)

	if err := indexer.AddDocuments([]model.Document{doc1, doc2}); err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	t.Run("no retrievable_fields specified - returns all fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrievableFields:        []string{}, // Empty means return all fields
		}
		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(result.Hits) == 0 {
			t.Fatal("Expected at least one hit")
		}

		hit := result.Hits[0]
		// Should contain all original fields
		expectedFields := []string{"documentID", "title", "description", "year", "rating", "director", "genre"}
		for _, field := range expectedFields {
			if _, exists := hit.Document[field]; !exists {
				t.Errorf("Expected field '%s' to be present in document, but it was missing", field)
			}
		}
	})

	t.Run("retrievable_fields specified - returns only specified fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrievableFields:        []string{"title", "year", "rating"}, // Only these fields should be returned
		}
		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(result.Hits) == 0 {
			t.Fatal("Expected at least one hit")
		}

		hit := result.Hits[0]

		// Should contain documentID (always included) plus specified fields
		expectedFields := []string{"documentID", "title", "year", "rating"}
		for _, field := range expectedFields {
			if _, exists := hit.Document[field]; !exists {
				t.Errorf("Expected field '%s' to be present in document, but it was missing", field)
			}
		}

		// Should NOT contain fields that were not specified
		unexpectedFields := []string{"description", "director", "genre"}
		for _, field := range unexpectedFields {
			if _, exists := hit.Document[field]; exists {
				t.Errorf("Expected field '%s' to be filtered out, but it was present", field)
			}
		}

		// Verify the document has exactly the expected number of fields
		expectedFieldCount := len(expectedFields)
		actualFieldCount := len(hit.Document)
		if actualFieldCount != expectedFieldCount {
			t.Errorf("Expected document to have %d fields, but got %d", expectedFieldCount, actualFieldCount)
		}
	})

	t.Run("retrievable_fields with documentID always included", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrievableFields:        []string{"title"}, // Only title specified, but documentID should still be included
		}
		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(result.Hits) == 0 {
			t.Fatal("Expected at least one hit")
		}

		hit := result.Hits[0]

		// Should contain documentID (always included) and title
		if _, exists := hit.Document["documentID"]; !exists {
			t.Error("Expected documentID to always be present")
		}
		if _, exists := hit.Document["title"]; !exists {
			t.Error("Expected title to be present")
		}

		// Should have exactly 2 fields: documentID and title
		if len(hit.Document) != 2 {
			t.Errorf("Expected document to have 2 fields (documentID + title), but got %d", len(hit.Document))
		}
	})

	t.Run("retrievable_fields with non-existent field", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrievableFields:        []string{"title", "nonexistent_field"}, // nonexistent_field should be ignored
		}
		result, err := service.Search(query)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(result.Hits) == 0 {
			t.Fatal("Expected at least one hit")
		}

		hit := result.Hits[0]

		// Should contain documentID and title, but not the non-existent field
		expectedFields := []string{"documentID", "title"}
		for _, field := range expectedFields {
			if _, exists := hit.Document[field]; !exists {
				t.Errorf("Expected field '%s' to be present", field)
			}
		}

		// Should not contain the non-existent field
		if _, exists := hit.Document["nonexistent_field"]; exists {
			t.Error("Non-existent field should not be present in result")
		}

		// Should have exactly 2 fields
		if len(hit.Document) != 2 {
			t.Errorf("Expected document to have 2 fields, but got %d", len(hit.Document))
		}
	})
}

// createTestService creates a test service with the given documents
func createTestService(t *testing.T, docs []model.Document) *Service {
	t.Helper()

	settings := &config.IndexSettings{
		Name:                      "test_multi_search",
		SearchableFields:          []string{"title", "content", "category"},
		FilterableFields:          []string{"year", "category"},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
		NoTypoToleranceFields:     []string{},
		DistinctField:             "",
		RankingCriteria:           []config.RankingCriterion{},
	}

	docStore := &store.DocumentStore{
		Docs:                   make(map[uint32]model.Document),
		ExternalIDtoInternalID: make(map[string]uint32),
		NextID:                 0,
	}

	invIndex := &index.InvertedIndex{
		Index:    make(map[string]index.PostingList),
		Settings: settings,
	}

	indexerService, err := indexing.NewService(invIndex, docStore)
	if err != nil {
		t.Fatalf("Failed to create indexer service: %v", err)
	}

	searchService, err := NewService(invIndex, docStore, settings)
	if err != nil {
		t.Fatalf("Failed to create search service: %v", err)
	}

	// Add documents to index
	err = indexerService.AddDocuments(docs)
	if err != nil {
		t.Fatalf("Failed to add documents: %v", err)
	}

	// Update typo finder after indexing
	searchService.UpdateTypoFinder()

	return searchService
}

func TestMultiSearch(t *testing.T) {
	// Setup test data
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "Go Programming Language",
			"content":    "Learn Go programming with examples",
			"category":   "programming",
			"year":       2020,
		},
		{
			"documentID": "doc2",
			"title":      "Python Programming",
			"content":    "Python is a versatile programming language",
			"category":   "programming",
			"year":       2019,
		},
		{
			"documentID": "doc3",
			"title":      "Web Development",
			"content":    "Building web applications with Go",
			"category":   "web",
			"year":       2021,
		},
	}

	// Create service with test data
	service := createTestService(t, docs)

	t.Run("separate queries execution", func(t *testing.T) {
		multiQuery := services.MultiSearchQuery{
			Queries: []services.NamedSearchQuery{
				{
					Name:                     "go_search",
					Query:                    "Go",
					RestrictSearchableFields: []string{"title", "content"},
				},
				{
					Name:                     "python_search",
					Query:                    "Python",
					RestrictSearchableFields: []string{"title", "content"},
				},
			},
			Page:     1,
			PageSize: 10,
		}

		result, err := service.MultiSearch(context.Background(), multiQuery)
		if err != nil {
			t.Fatalf("MultiSearch failed: %v", err)
		}

		// Should have results for both queries
		if len(result.Results) != 2 {
			t.Errorf("Expected 2 query results, got %d", len(result.Results))
		}

		// Check go_search results
		goResults, exists := result.Results["go_search"]
		if !exists {
			t.Error("Expected 'go_search' results")
		} else if len(goResults.Hits) == 0 {
			t.Error("Expected hits for 'go_search'")
		}

		// Check python_search results
		pythonResults, exists := result.Results["python_search"]
		if !exists {
			t.Error("Expected 'python_search' results")
		} else if len(pythonResults.Hits) == 0 {
			t.Error("Expected hits for 'python_search'")
		}

		// Check metadata
		if result.TotalQueries != 2 {
			t.Errorf("Expected TotalQueries=2, got %d", result.TotalQueries)
		}

		if result.ProcessingTimeMs <= 0 {
			t.Error("Expected positive processing time")
		}
	})

	t.Run("queries with filters", func(t *testing.T) {
		multiQuery := services.MultiSearchQuery{
			Queries: []services.NamedSearchQuery{
				{
					Name:  "programming_2020",
					Query: "programming",
					Filters: &services.Filters{
						Operator: "AND",
						Filters: []services.FilterCondition{
							{Field: "category", Value: "programming"},
							{Field: "year", Value: 2020},
						},
					},
				},
				{
					Name:  "web_category",
					Query: "web",
					Filters: &services.Filters{
						Operator: "AND",
						Filters: []services.FilterCondition{
							{Field: "category", Value: "web"},
						},
					},
				},
			},
		}

		result, err := service.MultiSearch(context.Background(), multiQuery)
		if err != nil {
			t.Fatalf("MultiSearch with filters failed: %v", err)
		}

		// Should have results for both queries
		if len(result.Results) != 2 {
			t.Errorf("Expected 2 query results, got %d", len(result.Results))
		}
	})

	t.Run("empty queries validation", func(t *testing.T) {
		multiQuery := services.MultiSearchQuery{
			Queries: []services.NamedSearchQuery{},
		}

		_, err := service.MultiSearch(context.Background(), multiQuery)
		if err == nil {
			t.Error("Expected error for empty queries")
		}
		if !strings.Contains(err.Error(), "at least one query is required") {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("empty query name validation", func(t *testing.T) {
		multiQuery := services.MultiSearchQuery{
			Queries: []services.NamedSearchQuery{
				{
					Name:  "",
					Query: "test",
				},
			},
		}

		_, err := service.MultiSearch(context.Background(), multiQuery)
		if err == nil {
			t.Error("Expected error for empty query name")
		}
		if !strings.Contains(err.Error(), "non-empty name") {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("query with typo tolerance overrides", func(t *testing.T) {
		multiQuery := services.MultiSearchQuery{
			Queries: []services.NamedSearchQuery{
				{
					Name:                "typo_search",
					Query:               "programing", // Typo: missing 'm'
					MinWordSizeFor1Typo: &[]int{3}[0], // Enable typo tolerance
				},
			},
		}

		result, err := service.MultiSearch(context.Background(), multiQuery)
		if err != nil {
			t.Fatalf("MultiSearch with typo tolerance failed: %v", err)
		}

		// Should have results
		if len(result.Results) != 1 {
			t.Errorf("Expected 1 query result, got %d", len(result.Results))
		}
	})
}

func TestNonTypoTolerantWords(t *testing.T) {
	// Add test documents
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "History of World War II",
			"content":    "A comprehensive study of the war including Hitler's role",
			"category":   "history",
		},
		{
			"documentID": "doc2",
			"title":      "Stalin's Soviet Union",
			"content":    "Biography of Joseph Stalin and his policies",
			"category":   "history",
		},
		{
			"documentID": "doc3",
			"title":      "COVID-19 Pandemic",
			"content":    "Analysis of the global pandemic response",
			"category":   "health",
		},
		{
			"documentID": "doc4",
			"title":      "World War II Documentary",
			"content":    "Documentary about the final days",
			"category":   "history",
		},
	}

	// Create test service and then modify its settings for non-typo tolerant words
	service := createTestService(t, docs)

	// Update the service settings to include non-typo tolerant words
	service.settings.NonTypoTolerantWords = []string{"hitler", "stalin", "covid"}
	service.settings.MinWordSizeFor1Typo = 3
	service.settings.MinWordSizeFor2Typos = 6

	// Test 1: Search for exact "hitler" should work (exact matches always work)
	query := services.SearchQuery{
		QueryString: "hitler",
		Page:        1,
		PageSize:    10,
	}

	result, err := service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total, "Should find exact matches for non-typo tolerant words")
	assert.Equal(t, "doc1", result.Hits[0].Document["documentID"])

	// Test 2: Search for "hitlar" should NOT match "hitler" via typos
	// because "hitler" is in the non-typo tolerant words list
	query.QueryString = "hitlar"
	result, err = service.Search(query)
	assert.NoError(t, err)

	// Debug: Print what we found
	if result.Total > 0 {
		t.Logf("Found %d results for 'hitlar':", result.Total)
		for i, hit := range result.Hits {
			t.Logf("  Hit %d: %v, Score: %f, FieldMatches: %v", i, hit.Document["documentID"], hit.Score, hit.FieldMatches)
		}
	}

	assert.Equal(t, 0, result.Total, "Should not find typo matches TO non-typo tolerant word 'hitler'")

	// Test 3: Search for "stalin" should work exactly
	query.QueryString = "stalin"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total, "Should find exact matches for 'stalin'")

	// Test 4: Search for "staln" should NOT match "stalin" via typos
	query.QueryString = "staln"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 0, result.Total, "Should not find typo matches TO non-typo tolerant word 'stalin'")

	// Test 5: Search for "covid" should work exactly
	query.QueryString = "covid"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total, "Should find exact matches for 'covid'")

	// Test 6: Search for "covd" should NOT match "covid" via typos
	query.QueryString = "covd"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 0, result.Total, "Should not find typo matches TO non-typo tolerant word 'covid'")

	// Test 7: Regular typo tolerance should still work for other words
	query.QueryString = "pandemc" // typo for "pandemic"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total, "Should find typo matches for regular words")
	assert.Equal(t, "doc3", result.Hits[0].Document["documentID"])

	// Test 8: Case insensitive exact matching should work
	query.QueryString = "HITLER"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total, "Should find exact matches for non-typo tolerant words (case insensitive)")

	// Test 9: Case insensitive typo prevention should work
	query.QueryString = "HITLAR"
	result, err = service.Search(query)
	assert.NoError(t, err)
	assert.Equal(t, 0, result.Total, "Should not find typo matches TO non-typo tolerant words (case insensitive)")

	// Test 10: Non-typo tolerant words should not generate typos FROM them either
	// If someone searches for "hitler", it should not generate typos like "hitlar", "hitleer", etc.
	// This is already handled by the first check in the typo logic
}

func TestMultiSearchParallel(t *testing.T) {
	// Add test documents
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "The Matrix",
			"content":    "A sci-fi movie",
			"category":   "movie",
		},
		{
			"documentID": "doc2",
			"title":      "Matrix Reloaded",
			"content":    "Sequel to The Matrix",
			"category":   "movie",
		},
		{
			"documentID": "doc3",
			"title":      "Science Fiction Guide",
			"content":    "A comprehensive guide",
			"category":   "book",
		},
	}

	service := createTestService(t, docs)

	// Test parallel execution with timing
	multiQuery := services.MultiSearchQuery{
		Queries: []services.NamedSearchQuery{
			{
				Name:                     "title_search",
				Query:                    "matrix",
				RestrictSearchableFields: []string{"title"},
			},
			{
				Name:                     "content_search",
				Query:                    "sci-fi",
				RestrictSearchableFields: []string{"content"},
			},
			{
				Name:  "filtered_search",
				Query: "matrix",
				Filters: &services.Filters{
					Operator: "AND",
					Filters: []services.FilterCondition{
						{Field: "category", Value: "movie"},
					},
				},
			},
		},
		Page:     1,
		PageSize: 10,
	}

	ctx := context.Background()
	startTime := time.Now()
	result, err := service.MultiSearch(ctx, multiQuery)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.TotalQueries)
	assert.Equal(t, 3, len(result.Results))

	// Verify all queries returned results
	titleResult, exists := result.Results["title_search"]
	assert.True(t, exists)
	assert.Equal(t, 2, titleResult.Total) // Both Matrix movies

	contentResult, exists := result.Results["content_search"]
	assert.True(t, exists)
	assert.Equal(t, 1, contentResult.Total) // One sci-fi movie

	filteredResult, exists := result.Results["filtered_search"]
	assert.True(t, exists)
	assert.Equal(t, 2, filteredResult.Total) // Both Matrix movies (category=movie)

	// Verify parallel execution was reasonably fast
	// (This is a rough check - parallel should be faster than sequential)
	assert.Less(t, duration.Milliseconds(), int64(100), "Parallel execution should be reasonably fast")
	assert.Greater(t, result.ProcessingTimeMs, 0.0, "Processing time should be recorded")
}

func TestMultiSearchContextCancellation(t *testing.T) {
	// Add a test document
	docs := []model.Document{
		{
			"documentID": "doc1",
			"title":      "Test Document",
		},
	}

	service := createTestService(t, docs)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	multiQuery := services.MultiSearchQuery{
		Queries: []services.NamedSearchQuery{
			{
				Name:  "test_search",
				Query: "test",
			},
		},
		Page:     1,
		PageSize: 10,
	}

	result, err := service.MultiSearch(ctx, multiQuery)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "multi-search cancelled")
}

// TestTypoToleranceOptimization tests the optimization where documents with exact matches
// for all query tokens skip typo processing, and verifies correct hit info reporting.
func TestTypoToleranceOptimization(t *testing.T) {
	// Create test documents
	docs := []model.Document{
		{
			"documentID": "exact_match_doc",
			"title":      "The Office Show",
			"castNames":  []string{"Steve Carell", "John Krasinski"},
			"crewNames":  []string{"Greg Daniels", "Michael Schur"},
		},
		{
			"documentID": "typo_match_doc",
			"title":      "The Offic Comedy", // "offic" is a typo for "office"
			"castNames":  []string{"Steve Carell"},
			"crewNames":  []string{"Greg Daniels"},
		},
		{
			"documentID": "partial_exact_doc",
			"title":      "The Officer Training", // has "the" exactly, "officer" as typo for "office"
			"castNames":  []string{"John Smith"},
			"crewNames":  []string{"Jane Doe"},
		},
	}

	// Set up service with specific settings
	settings := &config.IndexSettings{
		Name:                      "typo_test_index",
		SearchableFields:          []string{"title", "castNames", "crewNames"},
		FilterableFields:          []string{},
		RankingCriteria:           []config.RankingCriterion{{Field: "~score", Order: "desc"}},
		MinWordSizeFor1Typo:       4, // "office" (6 chars) qualifies for 1 typo
		MinWordSizeFor2Typos:      7, // "office" (6 chars) doesn't qualify for 2 typos
		FieldsWithoutPrefixSearch: []string{},
	}

	service, indexer := setupTestSearchService(t, settings)

	// Add documents to index
	err := indexer.AddDocuments(docs)
	assert.NoError(t, err, "Failed to add documents")

	t.Run("exact matches skip typo processing", func(t *testing.T) {
		// Search for "the office" - should find exact matches and skip typo processing for those docs
		query := services.SearchQuery{
			QueryString: "the office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")
		assert.Greater(t, result.Total, 0, "Should find at least one result")

		// Find the exact match document
		var exactMatchHit *services.HitResult
		var typoMatchHit *services.HitResult

		for i := range result.Hits {
			hit := &result.Hits[i]
			docID := hit.Document["documentID"].(string)
			switch docID {
			case "exact_match_doc":
				exactMatchHit = hit
			case "typo_match_doc":
				typoMatchHit = hit
			}
		}

		// Verify exact match document
		if exactMatchHit != nil {
			assert.Equal(t, 0, exactMatchHit.Info.NumTypos, "Exact match doc should have 0 typos")
			assert.Equal(t, 2, exactMatchHit.Info.NumberExactWords, "Exact match doc should have 2 exact words")

			// Verify field matches show exact terms (no "(typo)" suffix)
			for fieldName, matches := range exactMatchHit.FieldMatches {
				for _, match := range matches {
					assert.NotContains(t, match, "(typo)", "Exact matches should not have (typo) suffix in field %s", fieldName)
				}
			}
		}

		// Verify typo match document (if found)
		if typoMatchHit != nil {
			assert.Greater(t, typoMatchHit.Info.NumTypos, 0, "Typo match doc should have typos > 0")

			// Should show actual typo terms matched
			foundTypoTerm := false
			for fieldName, matches := range typoMatchHit.FieldMatches {
				for _, match := range matches {
					if strings.Contains(match, "(typo)") {
						foundTypoTerm = true
						// Should show the actual typo term, not the original query term
						assert.True(t, strings.HasPrefix(match, "offic(typo)") || strings.HasPrefix(match, "offici(typo)"),
							"Typo match should show actual typo term in field %s, got: %s", fieldName, match)
					}
				}
			}
			assert.True(t, foundTypoTerm, "Should find at least one typo term in field matches")
		}
	})

	t.Run("single token exact match optimization", func(t *testing.T) {
		// Search for just "office" - should find exact match and skip typo processing
		query := services.SearchQuery{
			QueryString: "office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Find the exact match document
		var exactMatchHit *services.HitResult
		for i := range result.Hits {
			hit := &result.Hits[i]
			if hit.Document["documentID"].(string) == "exact_match_doc" {
				exactMatchHit = hit
				break
			}
		}

		if exactMatchHit != nil {
			assert.Equal(t, 0, exactMatchHit.Info.NumTypos, "Single token exact match should have 0 typos")
			assert.Equal(t, 1, exactMatchHit.Info.NumberExactWords, "Single token exact match should have 1 exact word")
		}
	})

	t.Run("typo terms display correctly", func(t *testing.T) {
		// Search for a term that will generate typos
		query := services.SearchQuery{
			QueryString: "offic", // This should match "office" via typo tolerance
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		if result.Total > 0 {
			// At least one hit should show typo information
			foundTypoHit := false
			for _, hit := range result.Hits {
				if hit.Info.NumTypos > 0 {
					foundTypoHit = true

					// Verify that field matches show the actual typo term
					foundTypoInFieldMatches := false
					for fieldName, matches := range hit.FieldMatches {
						for _, match := range matches {
							if strings.Contains(match, "(typo)") {
								foundTypoInFieldMatches = true
								// Should not show "offic(typo)" but rather the actual matched term like "office(typo)"
								assert.NotEqual(t, "offic(typo)", match,
									"Should show actual matched typo term, not query term in field %s", fieldName)
							}
						}
					}
					assert.True(t, foundTypoInFieldMatches, "Typo hit should have typo terms in field matches")
				}
			}
			// Note: This assertion might not always pass depending on the indexed terms
			// but it's useful for verification when typos are found
			if !foundTypoHit {
				t.Log("No typo hits found - this may be expected depending on indexed terms")
			}
		}
	})

	t.Run("mixed exact and typo matches", func(t *testing.T) {
		// Search for terms where some docs have exact matches and others need typos
		query := services.SearchQuery{
			QueryString: "the offic", // "the" exact, "offic" needs typo tolerance
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Verify that we get different hit info for different documents
		exactMatchCount := 0
		typoMatchCount := 0

		for _, hit := range result.Hits {
			if hit.Info.NumTypos == 0 && hit.Info.NumberExactWords > 0 {
				exactMatchCount++
			} else if hit.Info.NumTypos > 0 {
				typoMatchCount++
			}
		}

		// We should have at least some results
		assert.Greater(t, exactMatchCount+typoMatchCount, 0, "Should have at least some matches")

		// Log the distribution for debugging
		t.Logf("Found %d exact matches and %d typo matches", exactMatchCount, typoMatchCount)
	})

	t.Run("performance optimization verification", func(t *testing.T) {
		// This test verifies that the optimization is working by checking that
		// documents with all exact matches don't get typo processing

		// Add a document that would match exactly
		perfTestDoc := model.Document{
			"documentID": "perf_test_doc",
			"title":      "Performance Test Document",
			"castNames":  []string{"Test Actor"},
			"crewNames":  []string{"Test Director"},
		}

		err := indexer.AddDocuments([]model.Document{perfTestDoc})
		assert.NoError(t, err, "Failed to add performance test document")

		// Search for exact terms
		query := services.SearchQuery{
			QueryString: "performance test",
			PageSize:    10,
		}

		startTime := time.Now()
		result, err := service.Search(query)
		duration := time.Since(startTime)

		assert.NoError(t, err, "Search should not error")
		assert.Greater(t, result.Total, 0, "Should find the performance test document")

		// Find our test document
		var testHit *services.HitResult
		for i := range result.Hits {
			hit := &result.Hits[i]
			if hit.Document["documentID"].(string) == "perf_test_doc" {
				testHit = hit
				break
			}
		}

		if testHit != nil {
			assert.Equal(t, 0, testHit.Info.NumTypos, "Performance test doc should have 0 typos")
			assert.Equal(t, 2, testHit.Info.NumberExactWords, "Performance test doc should have 2 exact words")
		}

		// The search should complete reasonably quickly (this is a rough performance check)
		assert.Less(t, duration.Milliseconds(), int64(1000), "Search should complete within 1 second")

		t.Logf("Search completed in %v", duration)
	})
}

// TestOriginalIssueScenario tests the specific scenario from the original issue:
// searching for "the office" should not show "office(typo)" for documents that have exact matches.
func TestOriginalIssueScenario(t *testing.T) {
	// Create documents similar to the original issue
	docs := []model.Document{
		{
			"documentID":      "office_show_doc",
			"title":           "The Office",
			"castNames":       []string{"Steve Carell", "John Krasinski", "Jenna Fischer"},
			"crewNames":       []string{"Greg Daniels", "Michael Schur"},
			"aiSynonymsTitle": "workplace comedy sitcom", // This should NOT be searched
		},
		{
			"documentID": "office_building_doc",
			"title":      "Office Building Management",
			"castNames":  []string{"John Smith"},
			"crewNames":  []string{"Jane Doe"},
		},
		{
			"documentID": "officer_doc",
			"title":      "Police Officer Training", // "officer" might match "office" via typo
			"castNames":  []string{"Tom Hanks"},
			"crewNames":  []string{"Steven Spielberg"},
		},
	}

	// Use the same searchable fields as mentioned in the conversation summary
	settings := &config.IndexSettings{
		Name:                      "original_issue_test_index",
		SearchableFields:          []string{"title", "castNames", "crewNames"}, // Note: aiSynonymsTitle is NOT included
		FilterableFields:          []string{},
		RankingCriteria:           []config.RankingCriterion{{Field: "~score", Order: "desc"}},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
	}

	service, indexer := setupTestSearchService(t, settings)

	// Add documents to index
	err := indexer.AddDocuments(docs)
	assert.NoError(t, err, "Failed to add documents")

	t.Run("the office search - exact matches should not show typo markers", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString: "the office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")
		assert.Greater(t, result.Total, 0, "Should find at least one result")

		// Check each hit
		for _, hit := range result.Hits {
			docID := hit.Document["documentID"].(string)

			// For documents that have exact matches for both "the" and "office"
			if docID == "office_show_doc" || docID == "office_building_doc" {
				// These should have exact matches, not typo matches
				assert.Equal(t, 0, hit.Info.NumTypos, "Document %s should have 0 typos", docID)
				assert.Equal(t, 2, hit.Info.NumberExactWords, "Document %s should have 2 exact words", docID)

				// Verify field matches don't contain incorrect typo markers
				for fieldName, matches := range hit.FieldMatches {
					for _, match := range matches {
						// The original issue was seeing "office(typo)" when it should be just "office"
						assert.NotEqual(t, "office(typo)", match,
							"Document %s field %s should not show 'office(typo)' for exact matches, got: %s",
							docID, fieldName, match)

						// Should not have any typo markers for exact matches
						if match == "office" || match == "the" {
							assert.NotContains(t, match, "(typo)",
								"Exact match '%s' should not have typo marker in document %s field %s",
								match, docID, fieldName)
						}
					}
				}
			}
		}
	})

	t.Run("verify aiSynonymsTitle is not searched", func(t *testing.T) {
		// Search for terms that exist in aiSynonymsTitle but not in searchable fields
		query := services.SearchQuery{
			QueryString: "workplace comedy sitcom",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should not find the office_show_doc because aiSynonymsTitle is not in searchable fields
		for _, hit := range result.Hits {
			docID := hit.Document["documentID"].(string)
			assert.NotEqual(t, "office_show_doc", docID,
				"Should not find office_show_doc when searching aiSynonymsTitle content")
		}
	})

	t.Run("typo tolerance still works for appropriate cases", func(t *testing.T) {
		// Search for "offic" which should match "office" via typo tolerance
		query := services.SearchQuery{
			QueryString: "offic",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should find documents via typo tolerance
		if result.Total > 0 {
			foundTypoMatch := false
			for _, hit := range result.Hits {
				if hit.Info.NumTypos > 0 {
					foundTypoMatch = true

					// Verify the typo term is displayed correctly
					for fieldName, matches := range hit.FieldMatches {
						for _, match := range matches {
							if strings.Contains(match, "(typo)") {
								// Should show the actual matched term, not the query term
								assert.NotEqual(t, "offic(typo)", match,
									"Should show actual matched typo term, not query term in field %s", fieldName)
							}
						}
					}
				}
			}

			if foundTypoMatch {
				t.Log("Successfully found typo matches for 'offic' query")
			} else {
				t.Log("No typo matches found - may be expected depending on indexed terms")
			}
		}
	})
}

// TestRankingCriteriaPriority tests that ranking criteria are applied before search relevance scores
func TestRankingCriteriaPriority(t *testing.T) {
	// Create documents with different ranking values but similar search relevance
	docs := []model.Document{
		{
			"documentID":  "high_rank_doc",
			"title":       "Office Management",
			"rankingSort": 9.5,
			"popularity":  100.0,
		},
		{
			"documentID":  "low_rank_doc",
			"title":       "Office Building",
			"rankingSort": 3.2,
			"popularity":  50.0,
		},
		{
			"documentID":  "mid_rank_doc",
			"title":       "Office Space",
			"rankingSort": 6.8,
			"popularity":  75.0,
		},
	}

	t.Run("ranking criteria prioritized over search score", func(t *testing.T) {
		// Set up service with rankingSort as primary criterion
		settings := &config.IndexSettings{
			Name:             "ranking_test_index",
			SearchableFields: []string{"title"},
			FilterableFields: []string{},
			RankingCriteria: []config.RankingCriterion{
				{Field: "rankingSort", Order: "desc"}, // Primary: rankingSort descending
				{Field: "~score", Order: "desc"},      // Secondary: search relevance
			},
			MinWordSizeFor1Typo:       4,
			MinWordSizeFor2Typos:      7,
			FieldsWithoutPrefixSearch: []string{},
		}

		service, indexer := setupTestSearchService(t, settings)
		err := indexer.AddDocuments(docs)
		assert.NoError(t, err, "Failed to add documents")

		// Search for "office" - all documents should match with similar relevance
		query := services.SearchQuery{
			QueryString: "office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")
		assert.Equal(t, 3, result.Total, "Should find all 3 documents")

		// Verify documents are sorted by rankingSort descending, not by search score
		expectedOrder := []string{"high_rank_doc", "mid_rank_doc", "low_rank_doc"}
		for i, hit := range result.Hits {
			docID := hit.Document["documentID"].(string)
			assert.Equal(t, expectedOrder[i], docID,
				"Document at position %d should be %s, got %s", i, expectedOrder[i], docID)
		}

		// Verify that all documents have similar search scores (since they all match "office")
		// but are ordered by rankingSort
		t.Logf("Search scores: %v", []float64{result.Hits[0].Score, result.Hits[1].Score, result.Hits[2].Score})
		t.Logf("Ranking sorts: %v", []float64{
			result.Hits[0].Document["rankingSort"].(float64),
			result.Hits[1].Document["rankingSort"].(float64),
			result.Hits[2].Document["rankingSort"].(float64),
		})
	})

	t.Run("~score criterion works correctly", func(t *testing.T) {
		// Set up service with ~score as primary criterion
		settings := &config.IndexSettings{
			Name:             "score_test_index",
			SearchableFields: []string{"title"},
			FilterableFields: []string{},
			RankingCriteria: []config.RankingCriterion{
				{Field: "~score", Order: "desc"},      // Primary: search relevance
				{Field: "rankingSort", Order: "desc"}, // Secondary: rankingSort
			},
			MinWordSizeFor1Typo:       4,
			MinWordSizeFor2Typos:      7,
			FieldsWithoutPrefixSearch: []string{},
		}

		service, indexer := setupTestSearchService(t, settings)
		err := indexer.AddDocuments(docs)
		assert.NoError(t, err, "Failed to add documents")

		// Search for "office"
		query := services.SearchQuery{
			QueryString: "office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")
		assert.Equal(t, 3, result.Total, "Should find all 3 documents")

		// With ~score as primary criterion, should sort by search relevance first
		// Since all have similar relevance, should then sort by rankingSort
		// This should produce the same order as the previous test since search scores are similar
		for i := 0; i < len(result.Hits)-1; i++ {
			currentScore := result.Hits[i].Score
			nextScore := result.Hits[i+1].Score

			if currentScore == nextScore {
				// If search scores are equal, should be sorted by rankingSort desc
				currentRanking := result.Hits[i].Document["rankingSort"].(float64)
				nextRanking := result.Hits[i+1].Document["rankingSort"].(float64)
				assert.GreaterOrEqual(t, currentRanking, nextRanking,
					"When search scores are equal, should sort by rankingSort descending")
			} else {
				// Search scores should be in descending order
				assert.Greater(t, currentScore, nextScore,
					"Search scores should be in descending order")
			}
		}
	})

	t.Run("multiple ranking criteria applied in order", func(t *testing.T) {
		// Add documents with same rankingSort but different popularity
		docsWithTies := []model.Document{
			{
				"documentID":  "tie_doc_1",
				"title":       "Office Work",
				"rankingSort": 5.0,  // Same ranking
				"popularity":  90.0, // Higher popularity
			},
			{
				"documentID":  "tie_doc_2",
				"title":       "Office Work",
				"rankingSort": 5.0,  // Same ranking
				"popularity":  60.0, // Lower popularity
			},
		}

		settings := &config.IndexSettings{
			Name:             "multi_criteria_test_index",
			SearchableFields: []string{"title"},
			FilterableFields: []string{},
			RankingCriteria: []config.RankingCriterion{
				{Field: "rankingSort", Order: "desc"}, // Primary
				{Field: "popularity", Order: "desc"},  // Secondary
				{Field: "~score", Order: "desc"},      // Tertiary
			},
			MinWordSizeFor1Typo:       4,
			MinWordSizeFor2Typos:      7,
			FieldsWithoutPrefixSearch: []string{},
		}

		service, indexer := setupTestSearchService(t, settings)
		err := indexer.AddDocuments(docsWithTies)
		assert.NoError(t, err, "Failed to add documents")

		query := services.SearchQuery{
			QueryString: "office",
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")
		assert.Equal(t, 2, result.Total, "Should find both documents")

		// Since rankingSort is the same, should sort by popularity desc
		assert.Equal(t, "tie_doc_1", result.Hits[0].Document["documentID"].(string),
			"Document with higher popularity should come first")
		assert.Equal(t, "tie_doc_2", result.Hits[1].Document["documentID"].(string),
			"Document with lower popularity should come second")
	})
}

// TestExactMatchesScoreHigherThanTypos verifies that documents with exact matches
// always get higher search relevance scores than documents with typo matches
func TestExactMatchesScoreHigherThanTypos(t *testing.T) {
	docs := []model.Document{
		{
			"documentID":  "exact_match_doc",
			"title":       "The Office Show",
			"rankingSort": 5.0, // Same ranking to isolate score comparison
		},
		{
			"documentID":  "typo_match_doc",
			"title":       "The Offic Building", // "offic" is a 1-typo match for "office"
			"rankingSort": 5.0,                  // Same ranking to isolate score comparison
		},
		{
			"documentID":  "multiple_typo_doc",
			"title":       "The Offce Space", // "offce" is a 1-typo match for "office"
			"rankingSort": 5.0,               // Same ranking to isolate score comparison
		},
	}

	settings := &config.IndexSettings{
		Name:             "score_test_index",
		SearchableFields: []string{"title"},
		FilterableFields: []string{},
		RankingCriteria: []config.RankingCriterion{
			{Field: "~score", Order: "desc"}, // Sort by search relevance to test scoring
		},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
	}

	service, indexer := setupTestSearchService(t, settings)
	err := indexer.AddDocuments(docs)
	assert.NoError(t, err, "Failed to add documents")

	// Search for "the office"
	query := services.SearchQuery{
		QueryString: "the office",
		PageSize:    10,
	}

	result, err := service.Search(query)
	assert.NoError(t, err, "Search should not error")

	// Log what we actually found for debugging
	t.Logf("Found %d documents:", result.Total)
	for i, hit := range result.Hits {
		t.Logf("  %d. %s (score: %.2f, typos: %d, exact: %d)",
			i+1, hit.Document["documentID"], hit.Score, hit.Info.NumTypos, hit.Info.NumberExactWords)
	}

	// We should find at least the exact match
	assert.GreaterOrEqual(t, result.Total, 1, "Should find at least the exact match document")

	// Find each document in results
	var exactMatchHit *services.HitResult
	var typoMatchHit *services.HitResult
	var multipleTypoHit *services.HitResult

	for i := range result.Hits {
		hit := &result.Hits[i]
		docID := hit.Document["documentID"].(string)
		switch docID {
		case "exact_match_doc":
			exactMatchHit = hit
		case "typo_match_doc":
			typoMatchHit = hit
		case "multiple_typo_doc":
			multipleTypoHit = hit
		}
	}

	// Verify exact match was found
	assert.NotNil(t, exactMatchHit, "Should find exact match document")
	if exactMatchHit == nil {
		return // Exit early if exact match not found
	}

	// Log scores for debugging
	t.Logf("Exact match score: %.2f (typos: %d, exact: %d)",
		exactMatchHit.Score, exactMatchHit.Info.NumTypos, exactMatchHit.Info.NumberExactWords)

	if typoMatchHit != nil {
		t.Logf("Typo match score: %.2f (typos: %d, exact: %d)",
			typoMatchHit.Score, typoMatchHit.Info.NumTypos, typoMatchHit.Info.NumberExactWords)

		// CRITICAL: Exact matches should ALWAYS score higher than typo matches
		assert.Greater(t, exactMatchHit.Score, typoMatchHit.Score,
			"Exact match should score higher than typo match")
	}

	if multipleTypoHit != nil {
		t.Logf("Multiple typo score: %.2f (typos: %d, exact: %d)",
			multipleTypoHit.Score, multipleTypoHit.Info.NumTypos, multipleTypoHit.Info.NumberExactWords)

		assert.Greater(t, exactMatchHit.Score, multipleTypoHit.Score,
			"Exact match should score higher than multiple typo matches")
	}

	// Verify hit info is correct for exact match
	assert.Equal(t, 0, exactMatchHit.Info.NumTypos, "Exact match should have 0 typos")
	assert.Equal(t, 2, exactMatchHit.Info.NumberExactWords, "Exact match should have 2 exact words")

	// If we found typo matches, verify their hit info
	if typoMatchHit != nil {
		assert.Greater(t, typoMatchHit.Info.NumTypos, 0, "Typo match should have > 0 typos")
		assert.Equal(t, 1, typoMatchHit.Info.NumberExactWords, "Typo match should have 1 exact word ('the')")
	}

	// Verify that the exact match appears first when sorted by score
	assert.Equal(t, "exact_match_doc", result.Hits[0].Document["documentID"].(string),
		"Exact match should appear first when sorted by search relevance")
}

// TestScoringWithKnownTypos tests scoring with documents that we know will match via typos
func TestScoringWithKnownTypos(t *testing.T) {
	docs := []model.Document{
		{
			"documentID":  "exact_match",
			"title":       "office work",
			"rankingSort": 5.0,
		},
		{
			"documentID":  "typo_match_1",
			"title":       "offic work", // 1-char deletion typo
			"rankingSort": 5.0,
		},
		{
			"documentID":  "typo_match_2",
			"title":       "oficce work", // 1-char insertion typo
			"rankingSort": 5.0,
		},
	}

	settings := &config.IndexSettings{
		Name:             "typo_score_test",
		SearchableFields: []string{"title"},
		FilterableFields: []string{},
		RankingCriteria: []config.RankingCriterion{
			{Field: "~score", Order: "desc"}, // Sort by search relevance
		},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
	}

	service, indexer := setupTestSearchService(t, settings)
	err := indexer.AddDocuments(docs)
	assert.NoError(t, err, "Failed to add documents")

	// Search for "office work"
	query := services.SearchQuery{
		QueryString: "office work",
		PageSize:    10,
	}

	result, err := service.Search(query)
	assert.NoError(t, err, "Search should not error")

	// Log results for debugging
	t.Logf("Found %d documents for 'office work':", result.Total)
	for i, hit := range result.Hits {
		t.Logf("  %d. %s: '%s' (score: %.2f, typos: %d, exact: %d)",
			i+1, hit.Document["documentID"], hit.Document["title"],
			hit.Score, hit.Info.NumTypos, hit.Info.NumberExactWords)
	}

	// Should find at least the exact match
	assert.GreaterOrEqual(t, result.Total, 1, "Should find at least one document")

	// Find the exact match
	var exactMatch *services.HitResult
	for i := range result.Hits {
		if result.Hits[i].Document["documentID"] == "exact_match" {
			exactMatch = &result.Hits[i]
			break
		}
	}

	assert.NotNil(t, exactMatch, "Should find exact match document")
	if exactMatch == nil {
		return // Exit early if exact match not found
	}
	assert.Equal(t, 0, exactMatch.Info.NumTypos, "Exact match should have 0 typos")
	assert.Equal(t, 2, exactMatch.Info.NumberExactWords, "Exact match should have 2 exact words")

	// Check if any typo matches were found
	var typoMatches []*services.HitResult
	for i := range result.Hits {
		if result.Hits[i].Info.NumTypos > 0 {
			typoMatches = append(typoMatches, &result.Hits[i])
		}
	}

	if len(typoMatches) > 0 {
		t.Logf("Found %d typo matches", len(typoMatches))
		for _, typoMatch := range typoMatches {
			// CRITICAL TEST: Exact match should always score higher than typo matches
			assert.Greater(t, exactMatch.Score, typoMatch.Score,
				"Exact match (%.2f) should score higher than typo match %s (%.2f)",
				exactMatch.Score, typoMatch.Document["documentID"], typoMatch.Score)
		}

		// Verify exact match appears first in results (since sorted by score desc)
		assert.Equal(t, "exact_match", result.Hits[0].Document["documentID"],
			"Exact match should appear first when sorted by search relevance")
	} else {
		t.Log("No typo matches found - this is okay, the test still validates exact match scoring")
	}
}

// TestFilters tests the new complex filter expressions with AND/OR logic
func TestFilters(t *testing.T) {
	// Create test documents with various filter values
	docs := []model.Document{
		{
			"documentID":     "movie1",
			"title":          "Action Movie",
			"genre":          "Action",
			"year":           2023,
			"rating":         8.5,
			"is_premium":     true,
			"filters":        []interface{}{"plat_pc_dev_computer", "prop_nbcuott", "content_format_longform"},
			"suggestionType": "programme",
		},
		{
			"documentID":     "movie2",
			"title":          "Comedy Movie",
			"genre":          "Comedy",
			"year":           2022,
			"rating":         7.5,
			"is_premium":     false,
			"filters":        []interface{}{"plat_dev_all", "prop_all", "content_format_trailer"},
			"suggestionType": "series",
		},
		{
			"documentID":     "movie3",
			"title":          "Drama Movie",
			"genre":          "Drama",
			"year":           2021,
			"rating":         9.0,
			"is_premium":     true,
			"filters":        []interface{}{"plat_pc_dev_computer", "prop_nbcuott", "content_format_longform"},
			"suggestionType": "programme",
		},
	}

	// Create service with filter expression support
	settings := &config.IndexSettings{
		Name:             "filter_expression_test",
		SearchableFields: []string{"title", "genre"},
		FilterableFields: []string{"genre", "year", "rating", "is_premium", "filters", "suggestionType"},
		RankingCriteria: []config.RankingCriterion{
			{Field: "~filters", Order: "desc"},
			{Field: "~score", Order: "desc"},
		},
		MinWordSizeFor1Typo:       4,
		MinWordSizeFor2Typos:      7,
		FieldsWithoutPrefixSearch: []string{},
	}

	service, indexer := setupTestSearchService(t, settings)
	err := indexer.AddDocuments(docs)
	assert.NoError(t, err, "Failed to add documents")

	t.Run("simple OR filter expression", func(t *testing.T) {
		// Test OR logic: match documents with either Action OR Comedy genre
		filterExpr := &services.Filters{
			Operator: "OR",
			Filters: []services.FilterCondition{
				{Field: "genre", Value: "Action", Score: 2.0},
				{Field: "genre", Value: "Comedy", Score: 1.5},
			},
		}

		query := services.SearchQuery{
			QueryString: "movie",
			Filters:     filterExpr,
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should find movie1 (Action) and movie2 (Comedy)
		assert.Equal(t, 2, result.Total, "Should find 2 documents matching OR condition")

		// Check filter scores
		for _, hit := range result.Hits {
			genre := hit.Document["genre"].(string)
			switch genre {
			case "Action":
				assert.Equal(t, 2.0, hit.Info.FilterScore, "Action movie should have filter score 2.0")
			case "Comedy":
				assert.Equal(t, 1.5, hit.Info.FilterScore, "Comedy movie should have filter score 1.5")
			}
		}
	})

	t.Run("simple AND filter expression", func(t *testing.T) {
		// Test AND logic: match documents with Action genre AND premium status
		filterExpr := &services.Filters{
			Operator: "AND",
			Filters: []services.FilterCondition{
				{Field: "genre", Value: "Action", Score: 2.0},
				{Field: "is_premium", Value: true, Score: 3.0},
			},
		}

		query := services.SearchQuery{
			QueryString: "movie",
			Filters:     filterExpr,
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should find only movie1 (Action AND premium)
		assert.Equal(t, 1, result.Total, "Should find 1 document matching AND condition")

		hit := result.Hits[0]
		assert.Equal(t, "movie1", hit.Document["documentID"], "Should find movie1")
		assert.Equal(t, 5.0, hit.Info.FilterScore, "Filter score should be 2.0 + 3.0 = 5.0")
	})

	t.Run("complex nested filter expression", func(t *testing.T) {
		// Test complex expression: (Action OR Comedy) AND (premium OR high rating)
		filterExpr := &services.Filters{
			Operator: "AND",
			Groups: []services.Filters{
				{
					Operator: "OR",
					Filters: []services.FilterCondition{
						{Field: "genre", Value: "Action", Score: 2.0},
						{Field: "genre", Value: "Comedy", Score: 1.5},
					},
				},
				{
					Operator: "OR",
					Filters: []services.FilterCondition{
						{Field: "is_premium", Value: true, Score: 3.0},
						{Field: "rating", Operator: "_gte", Value: 8.0, Score: 2.5},
					},
				},
			},
		}

		query := services.SearchQuery{
			QueryString: "movie",
			Filters:     filterExpr,
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should find:
		// - movie1: Action (2.0) + premium (3.0) + rating >= 8.0 (2.5) = 7.5
		//   (both conditions in the second OR group match: premium AND high rating)
		// - movie2: Comedy (1.5) + rating 7.5 (doesn't match >= 8.0) + not premium = only Comedy matches, so no second group match = fails AND
		// Actually, movie2 should NOT match because it doesn't satisfy the second AND group
		// Only movie1 should match
		assert.Equal(t, 1, result.Total, "Should find 1 document matching complex condition")

		hit := result.Hits[0]
		assert.Equal(t, "movie1", hit.Document["documentID"], "Should find movie1")
		assert.Equal(t, 7.5, hit.Info.FilterScore, "Filter score should be 2.0 + 3.0 + 2.5 = 7.5 (both OR conditions in second group match)")
	})

	t.Run("array field contains with scoring", func(t *testing.T) {
		// Test array contains with OR logic and scoring
		filterExpr := &services.Filters{
			Operator: "OR",
			Filters: []services.FilterCondition{
				{Field: "filters", Operator: "_contains", Value: "plat_pc_dev_computer", Score: 2.0},
				{Field: "filters", Operator: "_contains", Value: "plat_dev_all", Score: 1.0},
			},
		}

		query := services.SearchQuery{
			QueryString: "movie",
			Filters:     filterExpr,
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// Should find movie1 and movie3 (both have plat_pc_dev_computer) and movie2 (has plat_dev_all)
		assert.Equal(t, 3, result.Total, "Should find 3 documents matching array contains")

		for _, hit := range result.Hits {
			filters := hit.Document["filters"].([]interface{})
			containsPc := false
			containsAll := false
			for _, filter := range filters {
				if filterStr, ok := filter.(string); ok {
					if filterStr == "plat_pc_dev_computer" {
						containsPc = true
					}
					if filterStr == "plat_dev_all" {
						containsAll = true
					}
				}
			}

			if containsPc {
				assert.Equal(t, 2.0, hit.Info.FilterScore, "Documents with plat_pc_dev_computer should have filter score 2.0")
			} else if containsAll {
				assert.Equal(t, 1.0, hit.Info.FilterScore, "Documents with plat_dev_all should have filter score 1.0")
			}
		}
	})

	t.Run("algolia-inspired complex expression", func(t *testing.T) {
		// Inspired by the Algolia query: multiple OR conditions within an AND structure
		filterExpr := &services.Filters{
			Operator: "AND",
			Groups: []services.Filters{
				{
					Operator: "OR",
					Filters: []services.FilterCondition{
						{Field: "suggestionType", Value: "programme", Score: 0.5},
						{Field: "suggestionType", Value: "series", Score: 0.5},
					},
				},
				{
					Operator: "OR",
					Filters: []services.FilterCondition{
						{Field: "filters", Operator: "_contains", Value: "plat_pc_dev_computer", Score: 1.0},
						{Field: "filters", Operator: "_contains", Value: "plat_dev_all", Score: 1.0},
					},
				},
				{
					Operator: "OR",
					Filters: []services.FilterCondition{
						{Field: "filters", Operator: "_contains", Value: "prop_nbcuott", Score: 0.8},
						{Field: "filters", Operator: "_contains", Value: "prop_all", Score: 0.8},
					},
				},
			},
		}

		query := services.SearchQuery{
			QueryString: "movie",
			Filters:     filterExpr,
			PageSize:    10,
		}

		result, err := service.Search(query)
		assert.NoError(t, err, "Search should not error")

		// All documents should match this complex expression
		assert.Equal(t, 3, result.Total, "Should find all 3 documents matching complex Algolia-style expression")

		// Check that all have appropriate filter scores
		for _, hit := range result.Hits {
			assert.Greater(t, hit.Info.FilterScore, 0.0, "All documents should have positive filter scores")
			// Each document should have at least 0.5 + 1.0 + 0.8 = 2.3 points
			assert.GreaterOrEqual(t, hit.Info.FilterScore, 2.3, "All documents should have at least 2.3 filter score")
		}
	})

}

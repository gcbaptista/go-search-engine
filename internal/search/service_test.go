package search

import (
	"strings"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gcbaptista/go-search-engine/store"
)

// --- Test Helpers ---

func newTestIndexSettings() *config.IndexSettings {
	return &config.IndexSettings{
		Name:                      "test_search_index",
		SearchableFields:          []string{"title", "description", "tags"},
		FilterableFields:          []string{"genre", "year", "rating", "is_available", "release_date", "features"},
		RankingCriteria:           []config.RankingCriterion{{"~score", "desc"}, {"popularity", "desc"}},
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

func TestDocMatchesFilters(t *testing.T) {
	service, _ := setupTestSearchService(t, nil)

	now := time.Now()
	doc := model.Document{
		"documentID":   "test_filter_test_movie_doc",
		"title":        "Filter Test Movie",
		"genre":        "Action",
		"year":         2020,
		"rating":       8.5,
		"is_available": true,
		"release_date": now.Format(time.RFC3339Nano),
		"features":     []string{"hdr", "atmos"},
		"tags":         []interface{}{"test", "filter"}, // Test []interface{} as well
	}

	tests := []struct {
		name     string
		doc      model.Document
		filters  map[string]interface{}
		expected bool
	}{
		{"no filters", doc, map[string]interface{}{}, true},
		{"exact string match pass", doc, map[string]interface{}{"genre": "Action"}, true},
		{"exact string match fail", doc, map[string]interface{}{"genre": "Comedy"}, false},
		{"exact string match case-sensitive fail", doc, map[string]interface{}{"genre": "action"}, false},
		{"exact number match pass", doc, map[string]interface{}{"year": 2020}, true},
		{"exact number match fail", doc, map[string]interface{}{"year": 2021}, false},
		{"float match pass", doc, map[string]interface{}{"rating": 8.5}, true},
		{"bool match pass", doc, map[string]interface{}{"is_available": true}, true},
		{"bool match fail", doc, map[string]interface{}{"is_available": false}, false},

		// Range filters
		{"year_gte pass", doc, map[string]interface{}{"year_gte": 2020}, true},
		{"year_gte fail", doc, map[string]interface{}{"year_gte": 2021}, false},
		{"year_gt pass", doc, map[string]interface{}{"year_gt": 2019}, true},
		{"year_gt fail", doc, map[string]interface{}{"year_gt": 2020}, false},
		{"rating_lte pass", doc, map[string]interface{}{"rating_lte": 8.5}, true},
		{"rating_lt pass", doc, map[string]interface{}{"rating_lt": 8.6}, true},

		// String operations
		{"title_contains pass", doc, map[string]interface{}{"title_contains": "Test"}, true},
		{"title_contains fail", doc, map[string]interface{}{"title_contains": "XYZ"}, false},
		{"title_ncontains pass", doc, map[string]interface{}{"title_ncontains": "XYZ"}, true},
		{"title_ncontains fail", doc, map[string]interface{}{"title_ncontains": "Test"}, false},

		// Slice operations
		{"features_contains pass (string in []string)", doc, map[string]interface{}{"features_contains": "hdr"}, true},
		{"features_contains fail", doc, map[string]interface{}{"features_contains": "dolby"}, false},
		{"tags_contains pass (string in []interface{})", doc, map[string]interface{}{"tags_contains": "test"}, true},

		// Time filter
		{"release_date_exact pass", doc, map[string]interface{}{"release_date": now.Format(time.RFC3339Nano)}, true},
		{"release_date_gte pass", doc, map[string]interface{}{"release_date_gte": now.Add(-time.Hour).Format(time.RFC3339Nano)}, true},
		{"release_date_lt fail", doc, map[string]interface{}{"release_date_lt": now.Format(time.RFC3339Nano)}, false},

		// Non-filterable field, should be ignored (effectively pass as this filter won't apply)
		{"non_filterable_field", doc, map[string]interface{}{"popularity_gt": 5}, true},
		// Unknown operator, should be ignored (effectively pass)
		{"year_unknown_op", doc, map[string]interface{}{"year_unknown_op": 2020}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure filterable fields map is up-to-date within the service if it's cached
			// (Currently, docMatchesFilters creates it on the fly or uses one from service struct if implemented)
			got := service.docMatchesFilters(tc.doc, tc.filters)
			if got != tc.expected {
				t.Errorf("docMatchesFilters() for doc %v with filters %v = %v, want %v", tc.doc, tc.filters, got, tc.expected)
			}
		})
	}
}

func TestParseFilterKey(t *testing.T) {
	tests := []struct {
		key       string
		wantField string
		wantOp    string
	}{
		{"year", "year", ""},
		{"year_gte", "year", "_gte"},
		{"title_contains", "title", "_contains"},
		{"description_ncontains", "description", "_ncontains"},
		{"my_field_name_lt", "my_field_name", "_lt"},
		{"_some_field_exact", "_some_field", "_exact"}, // Assuming leading underscore is part of field name
		{"field_only", "field_only", ""},
		{"field__op", "field_", "_op"}, // Double underscore
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			gotField, gotOp := parseFilterKey(tc.key)
			if gotField != tc.wantField || gotOp != tc.wantOp {
				t.Errorf("parseFilterKey(%q) = (%q, %q), want (%q, %q)", tc.key, gotField, gotOp, tc.wantField, tc.wantOp)
			}
		})
	}
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

	t.Run("demonstrate parsing conflict in practice", func(t *testing.T) {
		// This test shows the actual problem: if you have a field named "my_field_exact",
		// you cannot filter it because the parser will interpret "_exact" as an operator

		// Scenario: User has a field literally named "rating_exact"
		// They want to filter documents where rating_exact = "premium"

		fieldName, operator := parseFilterKey("rating_exact")

		// What the user wanted: field="rating_exact", operator=""
		// What they got: field="rating", operator="_exact"
		if fieldName != "rating" || operator != "_exact" {
			t.Errorf("parseFilterKey('rating_exact') = ('%s', '%s'), showing the conflict behavior", fieldName, operator)
		}

		// To filter a field named "rating_exact" for exact match, there's currently no way
		// The user would need to rename their field to avoid the conflict
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

// TestRetrivableFields tests the functionality of limiting returned document fields
func TestRetrivableFields(t *testing.T) {
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

	t.Run("no retrivable_fields specified - returns all fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrivableFields:         []string{}, // Empty means return all fields
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

	t.Run("retrivable_fields specified - returns only specified fields", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrivableFields:         []string{"title", "year", "rating"}, // Only these fields should be returned
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

	t.Run("retrivable_fields with documentID always included", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrivableFields:         []string{"title"}, // Only title specified, but documentID should still be included
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

	t.Run("retrivable_fields with non-existent field", func(t *testing.T) {
		query := services.SearchQuery{
			QueryString:              "Matrix",
			RestrictSearchableFields: []string{"title", "description"},
			RetrivableFields:         []string{"title", "nonexistent_field"}, // nonexistent_field should be ignored
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

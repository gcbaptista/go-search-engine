package indexing

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/store"
)

// Helper to create a basic IndexSettings for tests
func newTestSettings() *config.IndexSettings {
	return &config.IndexSettings{
		Name:                 "test_index",
		SearchableFields:     []string{"title", "description", "tags"},
		FilterableFields:     []string{"genre", "year"},
		RankingCriteria:      []config.RankingCriterion{{Field: "~score", Order: "desc"}}, // Generic relevance score
		MinWordSizeFor1Typo:  4,
		MinWordSizeFor2Typos: 7,
		// Default: N-grams for "title", not for "description", "tags"
		FieldsWithoutPrefixSearch: []string{"description", "tags"},
	}
}

func TestNewService(t *testing.T) {
	t.Run("valid initialization", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestSettings(), Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		_, err := NewService(invIdx, docStore)
		if err != nil {
			t.Errorf("NewService() error = %v, wantErr nil", err)
		}
	})

	t.Run("nil inverted index", func(t *testing.T) {
		docStore := &store.DocumentStore{}
		_, err := NewService(nil, docStore)
		if err == nil {
			t.Error("NewService() with nil invertedIndex, wantErr, got nil")
		}
	})

	t.Run("nil document store", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestSettings()}
		_, err := NewService(invIdx, nil)
		if err == nil {
			t.Error("NewService() with nil documentStore, wantErr, got nil")
		}
	})

	t.Run("nil inverted index settings", func(t *testing.T) {
		invIdx := &index.InvertedIndex{} // Settings is nil
		docStore := &store.DocumentStore{}
		_, err := NewService(invIdx, docStore)
		if err == nil {
			t.Error("NewService() with nil invertedIndex.Settings, wantErr, got nil")
		}
	})

	t.Run("inverted index maps initialized if nil", func(t *testing.T) {
		invIdx := &index.InvertedIndex{Settings: newTestSettings()} // Index map is nil
		docStore := &store.DocumentStore{}                          // Docs and ExternalIDtoInternalID maps are nil
		s, err := NewService(invIdx, docStore)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if s.invertedIndex.Index == nil {
			t.Error("s.invertedIndex.Index was not initialized")
		}
		if s.documentStore.Docs == nil {
			t.Error("s.documentStore.Docs was not initialized")
		}
		if s.documentStore.ExternalIDtoInternalID == nil {
			t.Error("s.documentStore.ExternalIDtoInternalID was not initialized")
		}
	})
}

// Helper to check posting lists, ensuring they are sorted by score (desc)
func checkPostingList(t *testing.T, term string, pl index.PostingList, expectedEntries []index.PostingEntry) {
	t.Helper()
	if len(pl) != len(expectedEntries) {
		t.Errorf("Term %q: posting list len = %d, want %d. Got: %v, Want: %v", term, len(pl), len(expectedEntries), pl, expectedEntries)
		return
	}

	// Verify sort order by score (descending) - primary sort key of the indexer
	// Create a copy for checking sort order without modifying the original slice if it's not sorted as expected by the indexer
	plCopyForSortCheck := make(index.PostingList, len(pl))
	copy(plCopyForSortCheck, pl)
	sort.SliceStable(plCopyForSortCheck, func(i, j int) bool {
		if plCopyForSortCheck[i].Score != plCopyForSortCheck[j].Score {
			return plCopyForSortCheck[i].Score > plCopyForSortCheck[j].Score
		}
		if plCopyForSortCheck[i].DocID != plCopyForSortCheck[j].DocID {
			return plCopyForSortCheck[i].DocID < plCopyForSortCheck[j].DocID
		}
		return plCopyForSortCheck[i].FieldName < plCopyForSortCheck[j].FieldName
	})
	if !reflect.DeepEqual(pl, plCopyForSortCheck) {
		t.Errorf("Term %q: posting list not sorted by Score (desc) by indexer. Got: %v, Expected (after sorting for check): %v", term, pl, plCopyForSortCheck)
	}

	// For DeepEqual comparison, sort both actual and expected lists by a consistent key
	// (Score DESC, DocID ASC, FieldName ASC)
	// This makes the test robust to an implementation that might not preserve order for equal scores.
	sort.SliceStable(pl, func(i, j int) bool {
		if pl[i].Score != pl[j].Score {
			return pl[i].Score > pl[j].Score
		}
		if pl[i].DocID != pl[j].DocID {
			return pl[i].DocID < pl[j].DocID
		}
		return pl[i].FieldName < pl[j].FieldName
	})

	sort.SliceStable(expectedEntries, func(i, j int) bool {
		if expectedEntries[i].Score != expectedEntries[j].Score {
			return expectedEntries[i].Score > expectedEntries[j].Score
		}
		if expectedEntries[i].DocID != expectedEntries[j].DocID {
			return expectedEntries[i].DocID < expectedEntries[j].DocID
		}
		return expectedEntries[i].FieldName < expectedEntries[j].FieldName
	})

	// Explicitly cast pl (type index.PostingList) to []index.PostingEntry for DeepEqual
	actualEntriesForDeepEqual := []index.PostingEntry(pl)

	if !reflect.DeepEqual(actualEntriesForDeepEqual, expectedEntries) {
		t.Errorf("Term %q: reflect.DeepEqual failed. Dumping sorted lists:\nGot:  %#v (type %T)\nWant: %#v (type %T)", term, actualEntriesForDeepEqual, actualEntriesForDeepEqual, expectedEntries, expectedEntries)
		// Manual element-wise comparison for more detailed debugging
		for i := 0; i < len(pl); i++ {
			if !reflect.DeepEqual(pl[i], expectedEntries[i]) {
				t.Errorf("Term %q: Mismatch at index %d:\nGot:  %#v\nWant: %#v", term, i, pl[i], expectedEntries[i])
			}
		}
	}
}

func TestAddDocuments(t *testing.T) {
	docID1 := "test_doc_matrix_001"
	docID2 := "test_doc_matrix_reloaded_002"
	docID3 := "test_doc_product_003"

	baseDoc1 := model.Document{
		"documentID":  docID1,
		"title":       "The Matrix",
		"description": "A hacker learns about the true nature of reality.",
		"tags":        []string{"sci-fi", "action"},
		"popularity":  9.5,
		"genre":       "sci-fi",
		"year":        1999,
	}
	baseDoc2 := model.Document{
		"documentID":  docID2,
		"title":       "The Matrix Reloaded",
		"description": "Neo learns more.",
		// Using []interface{} for tags to test mixed types
		"tags":       []interface{}{"sci-fi", "sequel", "action"},
		"popularity": 8.7,
		"genre":      "sci-fi",
		"year":       2003,
	}

	t.Run("add multiple documents, ngrams for title, no ngrams for desc/tags", func(t *testing.T) {
		settings := newTestSettings() // title=ngram, desc/tags=no-ngram
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		docsToAdd := []model.Document{baseDoc1, baseDoc2}
		err := s.AddDocuments(docsToAdd)
		if err != nil {
			t.Fatalf("AddDocuments() error = %v", err)
		}

		// Check Document Store
		if len(docStore.Docs) != 2 {
			t.Errorf("Expected 2 documents in store, got %d", len(docStore.Docs))
		}
		if _, ok := docStore.Docs[0]; !ok {
			t.Fatal("Document with internal ID 0 not found")
		}
		if _, ok := docStore.Docs[1]; !ok {
			t.Fatal("Document with internal ID 1 not found")
		}
		if docStore.ExternalIDtoInternalID[docID1] != 0 {
			t.Errorf("External ID mapping incorrect for UUID %s", docID1)
		}
		if docStore.ExternalIDtoInternalID[docID2] != 1 {
			t.Errorf("External ID mapping incorrect for UUID %s", docID2)
		}

		// --- Document 1: baseDoc1 ---
		// Title: "The Matrix" (ngrams enabled) -> "the", "t", "th", "matrix", "m", "ma", "mat", "matr", "matri"
		// Description: "A hacker learns about the true nature of reality." (ngrams disabled)
		// -> "a", "hacker", "learns", "about", "the", "true", "nature", "of", "reality"
		// Tags: ["sci-fi", "action"] (ngrams disabled) -> "sci", "fi", "action"

		// Check "the"
		checkPostingList(t, "the", invIdx.Index["the"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0},       // from baseDoc1 title
			{DocID: 0, FieldName: "description", Score: 1.0}, // from baseDoc1 description
			{DocID: 1, FieldName: "title", Score: 1.0},       // from baseDoc2 title
		})
		// Check "matrix" (title, ngrams)
		checkPostingList(t, "matrix", invIdx.Index["matrix"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0}, // baseDoc1
			{DocID: 1, FieldName: "title", Score: 1.0}, // baseDoc2
		})
		checkPostingList(t, "m", invIdx.Index["m"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0}, // from matrix (doc0)
			{DocID: 1, FieldName: "title", Score: 1.0}, // from matrix (doc1)
			// {DocID: 1, FieldName: "description", Score: 1.0}, // from "more" (doc1 desc) - This setting has ngrams off for desc
		})

		// --- Document 2: baseDoc2 ---
		// Title: "The Matrix Reloaded" (ngrams enabled) -> "the", "t", "th", "matrix", "m", ..., "reloaded", "r", "re", ...
		// Description: "Neo learns more." (ngrams disabled) -> "neo", "learns", "more"
		// Tags: ["sci-fi", "sequel", "action"] (ngrams disabled) -> "sci", "fi", "sequel", "action"

		checkPostingList(t, "reloaded", invIdx.Index["reloaded"], []index.PostingEntry{
			{DocID: 1, FieldName: "title", Score: 1.0},
		})
		checkPostingList(t, "r", invIdx.Index["r"], []index.PostingEntry{ // Ngram from "reloaded"
			{DocID: 1, FieldName: "title", Score: 1.0},
		})
		checkPostingList(t, "neo", invIdx.Index["neo"], []index.PostingEntry{
			{DocID: 1, FieldName: "description", Score: 1.0},
		})
		checkPostingList(t, "learns", invIdx.Index["learns"], []index.PostingEntry{
			{DocID: 0, FieldName: "description", Score: 1.0}, // from baseDoc1
			{DocID: 1, FieldName: "description", Score: 1.0}, // from baseDoc2
		})
		checkPostingList(t, "sequel", invIdx.Index["sequel"], []index.PostingEntry{
			{DocID: 1, FieldName: "tags", Score: 1.0},
		})
		checkPostingList(t, "action", invIdx.Index["action"], []index.PostingEntry{
			{DocID: 0, FieldName: "tags", Score: 1.0}, // from baseDoc1
			{DocID: 1, FieldName: "tags", Score: 1.0}, // from baseDoc2
		})
		// This term "more" from baseDoc2 description (no ngrams for description)
		// Should not have "m" or "mo" from "more" if ngrams are off for description.
		checkPostingList(t, "more", invIdx.Index["more"], []index.PostingEntry{
			{DocID: 1, FieldName: "description", Score: 1.0},
		})
		// The 'm' from 'more' (desc, no ngrams) should not be here.
		// 'm' should only come from 'matrix' (title, ngrams enabled)
		plM := invIdx.Index["m"]
		var foundMFromMore bool
		for _, p := range plM {
			if p.DocID == 1 && p.FieldName == "description" {
				foundMFromMore = true
				break
			}
		}
		if foundMFromMore {
			t.Errorf("Token 'm' should not be indexed from 'more' in description (docID 1) as ngrams are off for description. Index['m']: %v", plM)
		}
	})

	t.Run("add two documents, ngrams for description, no ngrams for title/tags, check updates", func(t *testing.T) {
		settings := newTestSettings()
		// Ngrams on description, NOT on title/tags
		settings.FieldsWithoutPrefixSearch = []string{"title", "tags"}
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		doc1 := model.Document{
			"documentID":  docID1,
			"title":       "Movie Alpha",       // "movie", "alpha" (no ngrams)
			"description": "Alpha test movie.", // "alpha", "a",..., "test", "t",..., "movie", "m",... (ngrams)
			"tags":        []string{"test"},    // "test" (no ngrams)
			"popularity":  10.0,
		}
		doc2 := model.Document{
			"documentID":  docID2,
			"title":       "Movie Bravo",             // "movie", "bravo" (no ngrams)
			"description": "Bravo test movie.",       // "bravo", "b",..., "test", "t",..., "movie", "m",... (ngrams)
			"tags":        []string{"test", "movie"}, // "test", "movie" (no ngrams)
			"popularity":  9.0,
		}

		err := s.AddDocuments([]model.Document{doc1, doc2})
		if err != nil {
			t.Fatalf("AddDocuments() error = %v", err)
		}

		// Doc Store checks
		if len(docStore.Docs) != 2 {
			t.Fatalf("Expected 2 documents in store, got %d", len(docStore.Docs))
		}

		// Inverted Index checks
		// "movie": title(d0,TF1), desc(d0,TF1), title(d1,TF1), desc(d1,TF1), tags(d1,TF1)
		checkPostingList(t, "movie", invIdx.Index["movie"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0},
			{DocID: 0, FieldName: "description", Score: 1.0},
			{DocID: 1, FieldName: "title", Score: 1.0},
			{DocID: 1, FieldName: "description", Score: 1.0},
			{DocID: 1, FieldName: "tags", Score: 1.0},
		})
		// "alpha": title(d0,TF1), desc(d0,TF1)
		checkPostingList(t, "alpha", invIdx.Index["alpha"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0},
			{DocID: 0, FieldName: "description", Score: 1.0},
		})
		// Ngram "a" from description "Alpha test movie." of doc0 (ngrams on for description)
		checkPostingList(t, "a", invIdx.Index["a"], []index.PostingEntry{
			{DocID: 0, FieldName: "description", Score: 1.0}, // From alpha
		})
		// Ngram "b" from description "Bravo test movie." of doc1 (ngrams on for description)
		checkPostingList(t, "b", invIdx.Index["b"], []index.PostingEntry{
			{DocID: 1, FieldName: "description", Score: 1.0}, // From bravo
		})
		// Token "t" from title "Movie Alpha" should NOT exist if ngrams off for title.
		// Only from "test" in description (doc0, doc1) and "test" in tags (doc0, doc1 - but tags also no ngrams)
		// So "t" should only come from description's "test".
		plT := invIdx.Index["t"]
		var foundTFromTitleOrTags bool
		for _, p := range plT {
			if p.FieldName == "title" || p.FieldName == "tags" {
				foundTFromTitleOrTags = true
				break
			}
		}
		if foundTFromTitleOrTags {
			t.Errorf("Token 't' should not be indexed from title or tags as ngrams are off. Index['t']: %v", plT)
		}
		// Check "t" comes from description
		checkPostingList(t, "t", plT, []index.PostingEntry{
			{DocID: 0, FieldName: "description", Score: 1.0}, // from test in desc
			{DocID: 1, FieldName: "description", Score: 1.0}, // from test in desc
		})

		// Update doc1 (uuid1, internal ID 0)
		updatedDoc1 := model.Document{
			"documentID":  docID1,
			"title":       "Movie Alpha Remixed", // "movie", "alpha", "remixed" (no ngrams)
			"description": "Alpha is new.",       // "alpha", "a",..., "is", "i", "new", "n",... (ngrams)
			"tags":        []string{"updated"},   // "updated" (no ngrams)
			"popularity":  11.0,
		}
		err = s.AddDocuments([]model.Document{updatedDoc1})
		if err != nil {
			t.Fatalf("AddDocuments() for update error = %v", err)
		}
		if docStore.Docs[0]["popularity"].(float64) != 11.0 {
			t.Errorf("Document 0 popularity not updated. Got %v", docStore.Docs[0]["popularity"].(float64))
		}

		// Check "alpha" after update
		checkPostingList(t, "alpha", invIdx.Index["alpha"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0},       // from updatedDoc1 title
			{DocID: 0, FieldName: "description", Score: 1.0}, // from updatedDoc1 description
		})
		// Check "movie" after update
		checkPostingList(t, "movie", invIdx.Index["movie"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0}, // From updatedDoc1 title
			// Doc0 description no longer has "movie"
			{DocID: 1, FieldName: "title", Score: 1.0},       // From doc2 title
			{DocID: 1, FieldName: "description", Score: 1.0}, // From doc2 description (still has "movie", ngrams on)
			{DocID: 1, FieldName: "tags", Score: 1.0},        // From doc2 tags
		})
		// Ngram "i" from description "is" of updatedDoc1 (description has ngrams)
		checkPostingList(t, "i", invIdx.Index["i"], []index.PostingEntry{
			{DocID: 0, FieldName: "description", Score: 1.0},
		})
		// "remixed" from updatedDoc1 title (no ngrams for title)
		checkPostingList(t, "remixed", invIdx.Index["remixed"], []index.PostingEntry{
			{DocID: 0, FieldName: "title", Score: 1.0},
		})
		// "test" should now only have entries for doc1 (internal ID 1) from its description and tags
		checkPostingList(t, "test", invIdx.Index["test"], []index.PostingEntry{
			{DocID: 1, FieldName: "description", Score: 1.0}, // from doc2 description
			{DocID: 1, FieldName: "tags", Score: 1.0},        // from doc2 tags
		})
	})

	t.Run("documentID handling", func(t *testing.T) {
		settings := newTestSettings()
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		validUUID := "valid_uuid_string"
		docs := []model.Document{
			{"documentID": validUUID, "title": "Valid UUID string"},
			{"documentID": "valid_uuid_type", "title": "Valid UUID type"},
		}
		err := s.AddDocuments(docs)
		if err != nil {
			t.Errorf("AddDocuments with valid UUIDs failed: %v", err)
		}
		if len(docStore.Docs) != 2 {
			t.Errorf("Expected 2 docs, got %d", len(docStore.Docs))
		}

		// Invalid documentIDs (empty strings)
		invalidDocs := []model.Document{
			{"documentID": "", "title": "Empty documentID"},
		}
		err = s.AddDocuments(invalidDocs)
		if err == nil {
			t.Error("AddDocuments with empty documentID: expected error, got nil")
		} else if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("AddDocuments error mismatch for empty documentID. Got: %v", err)
		}

		whitespaceDocs := []model.Document{
			{"documentID": "   ", "title": "Whitespace documentID"},
		}
		err = s.AddDocuments(whitespaceDocs)
		if err == nil {
			t.Error("AddDocuments with whitespace-only documentID: expected error, got nil")
		} else if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("AddDocuments error mismatch for whitespace documentID. Got: %v", err)
		}

		noUUIDDocs := []model.Document{
			{"title": "Missing documentID"},
		}
		err = s.AddDocuments(noUUIDDocs)
		if err == nil {
			t.Error("AddDocuments with missing documentID: expected error, got nil")
		} else if !strings.Contains(err.Error(), "documentID not found") {
			t.Errorf("AddDocuments error mismatch for missing documentID. Got: %v", err)
		}

		wrongTypeUUIDDocs := []model.Document{
			{"documentID": 123, "title": "Wrong documentID type"},
		}
		err = s.AddDocuments(wrongTypeUUIDDocs)
		if err == nil {
			t.Error("AddDocuments with wrong documentID type: expected error, got nil")
		} else if !strings.Contains(err.Error(), "invalid type in the map") {
			t.Errorf("AddDocuments error mismatch for wrong documentID type. Got: %v", err)
		}
	})

	t.Run("field types and ngrams for all searchable fields", func(t *testing.T) {
		settings := newTestSettings()
		settings.FieldsWithoutPrefixSearch = []string{} // Ngrams for ALL searchable fields
		settings.SearchableFields = []string{"name", "categories", "notes"}
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		docWithFieldTypes := model.Document{
			"documentID":   docID3,
			"name":         "Product X",                      // string
			"categories":   []string{"tech", "gadget"},       // []string
			"notes":        []interface{}{"cool", "feature"}, // []interface{}
			"ignoredField": "should not be indexed",
		}
		err := s.AddDocuments([]model.Document{docWithFieldTypes})
		if err != nil {
			t.Fatalf("AddDocuments error: %v", err)
		}

		// Name: "Product X" -> "product", "p", "pr", ..., "x" (all ngrams)
		checkPostingList(t, "product", invIdx.Index["product"], []index.PostingEntry{{DocID: 0, FieldName: "name", Score: 1.0}})
		checkPostingList(t, "p", invIdx.Index["p"], []index.PostingEntry{{DocID: 0, FieldName: "name", Score: 1.0}}) // Ngram of "product"
		checkPostingList(t, "x", invIdx.Index["x"], []index.PostingEntry{{DocID: 0, FieldName: "name", Score: 1.0}}) // Full token "x" and its ngrams (just "x")

		// Categories: "tech gadget" -> "tech", "t", ..., "gadget", "g", ... (all ngrams)
		checkPostingList(t, "tech", invIdx.Index["tech"], []index.PostingEntry{{DocID: 0, FieldName: "categories", Score: 1.0}})
		// "t" from "tech" (categories)
		checkPostingList(t, "t", invIdx.Index["t"], []index.PostingEntry{
			{DocID: 0, FieldName: "categories", Score: 1.0}, // from tech
		})
		checkPostingList(t, "gadget", invIdx.Index["gadget"], []index.PostingEntry{{DocID: 0, FieldName: "categories", Score: 1.0}})

		// Notes: "cool feature" -> "cool", "c", ..., "feature", "f", ... (all ngrams)
		checkPostingList(t, "cool", invIdx.Index["cool"], []index.PostingEntry{{DocID: 0, FieldName: "notes", Score: 1.0}})
		// "c" from "cool" (notes) - "tech" does not produce a standalone "c" ngram
		checkPostingList(t, "c", invIdx.Index["c"], []index.PostingEntry{
			{DocID: 0, FieldName: "notes", Score: 1.0}, // from cool
		})
		checkPostingList(t, "feature", invIdx.Index["feature"], []index.PostingEntry{{DocID: 0, FieldName: "notes", Score: 1.0}})

		// Ignored field
		if _, exists := invIdx.Index["ignored"]; exists {
			t.Error("Token 'ignored' from non-searchable field found in index")
		}
	})

	t.Run("document with field having empty string content", func(t *testing.T) {
		settings := newTestSettings()
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		docWithEmptyField := model.Document{
			"documentID":  docID1,
			"title":       "Title Present",
			"description": "   ", // Empty after trim
			"tags":        []string{"tag1"},
		}

		err := s.AddDocuments([]model.Document{docWithEmptyField})
		if err != nil {
			t.Fatalf("AddDocuments error: %v", err)
		}

		// Check that description field did not add any tokens
		for token, pl := range invIdx.Index {
			for _, entry := range pl {
				if entry.FieldName == "description" {
					t.Errorf("Found token '%s' from empty description field: %v", token, entry)
				}
			}
		}
		// Ensure title and tags are indexed
		if _, ok := invIdx.Index["title"]; !ok { // "title" token from "Title Present" (ngrams for title)
			t.Error("Token 'title' not found from title field")
		}
		if _, ok := invIdx.Index["present"]; !ok { // "present" token from "Title Present" (ngrams for title)
			t.Error("Token 'present' not found from title field")
		}
		if _, ok := invIdx.Index["tag1"]; !ok { // "tag1" token from tags (no ngrams for tags by default)
			t.Error("Token 'tag1' not found from tags field")
		}
	})

	t.Run("document with non-existent searchable field", func(t *testing.T) {
		settings := newTestSettings()
		settings.SearchableFields = []string{"title", "author"} // 'author' may not exist
		invIdx := &index.InvertedIndex{Settings: settings, Index: make(map[string]index.PostingList)}
		docStore := &store.DocumentStore{Docs: make(map[uint32]model.Document), ExternalIDtoInternalID: make(map[string]uint32)}
		s, _ := NewService(invIdx, docStore)

		docMissingField := model.Document{
			"documentID": docID1,
			"title":      "A Good Book",
			// "author" field is missing
		}

		err := s.AddDocuments([]model.Document{docMissingField})
		if err != nil {
			t.Fatalf("AddDocuments error: %v", err)
		}

		if _, ok := invIdx.Index["good"]; !ok { // "good" from title (ngrams on for title by default)
			t.Error("Token 'good' from title not found when other searchable field is missing")
		}
		if _, ok := invIdx.Index["g"]; !ok { // ngram "g" from "good"
			t.Error("Ngram 'g' from title not found")
		}

		for token, pl := range invIdx.Index {
			for _, entry := range pl {
				if entry.FieldName == "author" {
					t.Errorf("Found token '%s' from missing 'author' field: %v", token, entry)
				}
			}
		}
	})
}

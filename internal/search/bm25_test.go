package search

import (
	"testing"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/store"
)

func TestBM25Calculator(t *testing.T) {
	// Create test data
	settings := &config.IndexSettings{
		Name:             "test_bm25",
		SearchableFields: []string{"title", "description"},
		FilterableFields: []string{},
	}

	// Create inverted index and document store
	invertedIndex := &index.InvertedIndex{
		Index:    make(map[string]index.PostingList),
		Settings: settings,
	}

	documentStore := &store.DocumentStore{
		Docs:                   make(map[uint32]model.Document),
		ExternalIDtoInternalID: make(map[string]uint32),
		NextID:                 0,
	}

	// Add test documents with varying lengths
	docs := []model.Document{
		{
			"documentID":  "doc1",
			"title":       "The quick brown fox", // 4 words
			"description": "A story about a fox", // 5 words (total: 9)
		},
		{
			"documentID":  "doc2",
			"title":       "The brown dog",                                           // 3 words
			"description": "A very long story about a dog and fox with many details", // 12 words (total: 15)
		},
		{
			"documentID":  "doc3",
			"title":       "Quick reference guide",       // 3 words
			"description": "A guide for quick reference", // 5 words (total: 8)
		},
	}

	// Manually add documents to stores (simplified for testing)
	for i, doc := range docs {
		docID := uint32(i)
		documentStore.Docs[docID] = doc
		documentStore.ExternalIDtoInternalID[doc["documentID"].(string)] = docID
	}
	documentStore.NextID = uint32(len(docs))

	// Manually add some terms to inverted index for testing
	invertedIndex.Index["quick"] = index.PostingList{
		{DocID: 0, FieldName: "title", Score: 1.0},       // doc1: "quick" appears once in title
		{DocID: 2, FieldName: "title", Score: 1.0},       // doc3: "quick" appears once in title
		{DocID: 2, FieldName: "description", Score: 1.0}, // doc3: "quick" appears once in description
	}

	invertedIndex.Index["brown"] = index.PostingList{
		{DocID: 0, FieldName: "title", Score: 1.0}, // doc1: "brown" appears once in title
		{DocID: 1, FieldName: "title", Score: 1.0}, // doc2: "brown" appears once in title
	}

	invertedIndex.Index["fox"] = index.PostingList{
		{DocID: 0, FieldName: "title", Score: 1.0},       // doc1: "fox" appears once in title
		{DocID: 0, FieldName: "description", Score: 1.0}, // doc1: "fox" appears once in description
		{DocID: 1, FieldName: "description", Score: 1.0}, // doc2: "fox" appears once in description
	}

	// Create BM25 calculator
	bm25Calc := NewBM25Calculator(invertedIndex, documentStore)

	t.Run("IDF calculation", func(t *testing.T) {
		// Test IDF calculation: log(N / df)
		// Total documents = 3

		// "quick" appears in 2 documents (doc1, doc3)
		idfQuick := bm25Calc.calculateIDF("quick")
		expectedQuick := 0.4054651081081644 // log(3/2) ≈ 0.405
		if idfQuick != expectedQuick {
			t.Errorf("Expected IDF for 'quick' to be %f, got %f", expectedQuick, idfQuick)
		}

		// Non-existent term should return 0
		idfNonExistent := bm25Calc.calculateIDF("nonexistent")
		if idfNonExistent != 0.0 {
			t.Errorf("Expected IDF for non-existent term to be 0.0, got %f", idfNonExistent)
		}
	})

	t.Run("Document length calculation", func(t *testing.T) {
		searchableFields := []string{"title", "description"}

		// Test document lengths
		doc1Length := bm25Calc.getDocumentLength(documentStore.Docs[0], searchableFields)
		if doc1Length != 9 {
			t.Errorf("Expected doc1 length to be 9, got %d", doc1Length)
		}

		doc2Length := bm25Calc.getDocumentLength(documentStore.Docs[1], searchableFields)
		if doc2Length != 15 {
			t.Errorf("Expected doc2 length to be 15, got %d", doc2Length)
		}

		doc3Length := bm25Calc.getDocumentLength(documentStore.Docs[2], searchableFields)
		if doc3Length != 8 {
			t.Errorf("Expected doc3 length to be 8, got %d", doc3Length)
		}

		// Test average document length
		avgLength := bm25Calc.getAverageDocumentLength(searchableFields)
		expectedAvg := (9.0 + 15.0 + 8.0) / 3.0 // 32/3 ≈ 10.67
		if avgLength != expectedAvg {
			t.Errorf("Expected average document length to be %f, got %f", expectedAvg, avgLength)
		}
	})

	t.Run("BM25 calculation", func(t *testing.T) {
		searchableFields := []string{"title", "description"}

		// Test BM25 scoring for "quick" in different documents
		// Doc1 (length 9): should get higher score than doc2 (length 15) due to length normalization
		bm25Doc1 := bm25Calc.CalculateBM25("quick", 0, 1.0, searchableFields)
		bm25Doc2 := bm25Calc.CalculateBM25("quick", 1, 1.0, searchableFields) // "quick" doesn't appear in doc2, but testing the calculation

		t.Logf("BM25 score for 'quick' in doc1 (length 9): %f", bm25Doc1)
		t.Logf("BM25 score for 'quick' in doc2 (length 15): %f", bm25Doc2)

		// BM25 should be positive for existing terms
		if bm25Doc1 <= 0 {
			t.Errorf("Expected BM25 score for doc1 to be positive, got %f", bm25Doc1)
		}

		// Test with higher term frequency
		bm25HighFreq := bm25Calc.CalculateBM25("quick", 0, 3.0, searchableFields)
		t.Logf("BM25 score for 'quick' with freq=3.0: %f", bm25HighFreq)

		// Higher frequency should give higher score, but with saturation
		if bm25HighFreq <= bm25Doc1 {
			t.Errorf("Expected higher frequency to give higher score: %f vs %f", bm25HighFreq, bm25Doc1)
		}

		// But the increase should be less than linear due to BM25 saturation
		freqRatio := 3.0 / 1.0
		scoreRatio := bm25HighFreq / bm25Doc1
		if scoreRatio >= freqRatio {
			t.Errorf("Expected BM25 saturation effect: score ratio (%f) should be less than freq ratio (%f)", scoreRatio, freqRatio)
		}
	})

	t.Run("Document frequency calculation", func(t *testing.T) {
		// Test document frequency calculation
		dfQuick := bm25Calc.getDocumentFrequency("quick")
		if dfQuick != 2 {
			t.Errorf("Expected document frequency for 'quick' to be 2, got %d", dfQuick)
		}

		dfBrown := bm25Calc.getDocumentFrequency("brown")
		if dfBrown != 2 {
			t.Errorf("Expected document frequency for 'brown' to be 2, got %d", dfBrown)
		}

		dfFox := bm25Calc.getDocumentFrequency("fox")
		if dfFox != 2 {
			t.Errorf("Expected document frequency for 'fox' to be 2, got %d", dfFox)
		}

		dfNonExistent := bm25Calc.getDocumentFrequency("nonexistent")
		if dfNonExistent != 0 {
			t.Errorf("Expected document frequency for non-existent term to be 0, got %d", dfNonExistent)
		}
	})

	t.Run("BM25 vs document length", func(t *testing.T) {
		searchableFields := []string{"title", "description"}

		// Compare BM25 scores for the same term in documents of different lengths
		// "brown" appears in both doc1 (length 9) and doc2 (length 15)
		bm25Short := bm25Calc.CalculateBM25("brown", 0, 1.0, searchableFields) // doc1: shorter
		bm25Long := bm25Calc.CalculateBM25("brown", 1, 1.0, searchableFields)  // doc2: longer

		t.Logf("BM25 for 'brown' in short doc (9 words): %f", bm25Short)
		t.Logf("BM25 for 'brown' in long doc (15 words): %f", bm25Long)

		// Shorter document should get higher score due to BM25 length normalization
		if bm25Short <= bm25Long {
			t.Errorf("Expected shorter document to get higher BM25 score: %f vs %f", bm25Short, bm25Long)
		}
	})
}

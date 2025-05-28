package search

import (
	"math"

	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/store"
)

// BM25Calculator handles BM25 score calculations
type BM25Calculator struct {
	invertedIndex *index.InvertedIndex
	documentStore *store.DocumentStore
}

// NewBM25Calculator creates a new BM25 calculator
func NewBM25Calculator(invIndex *index.InvertedIndex, docStore *store.DocumentStore) *BM25Calculator {
	return &BM25Calculator{
		invertedIndex: invIndex,
		documentStore: docStore,
	}
}

// calculateIDF calculates the inverse document frequency
// IDF = log(N / df) where N = total documents, df = documents containing term
func (calc *BM25Calculator) calculateIDF(term string) float64 {
	// Get total number of documents
	totalDocs := float64(len(calc.documentStore.Docs))
	if totalDocs == 0 {
		return 0.0
	}

	// Get document frequency (number of documents containing this term)
	docFreq := calc.getDocumentFrequency(term)
	if docFreq == 0 {
		return 0.0
	}

	// IDF = log(N / df)
	return math.Log(totalDocs / float64(docFreq))
}

// getDocumentFrequency returns the number of documents that contain the given term
func (calc *BM25Calculator) getDocumentFrequency(term string) int {
	postingList, exists := calc.invertedIndex.Index[term]
	if !exists {
		return 0
	}

	// Count unique documents (a term might appear in multiple fields of the same document)
	uniqueDocs := make(map[uint32]bool)
	for _, entry := range postingList {
		uniqueDocs[entry.DocID] = true
	}

	return len(uniqueDocs)
}

// CalculateBM25 calculates BM25 score with document length normalization
// BM25 = IDF * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (|d| / avgdl)))
func (calc *BM25Calculator) CalculateBM25(term string, docID uint32, termFreq float64, searchableFields []string) float64 {
	// BM25 parameters
	k1 := 1.2 // Controls term frequency saturation
	b := 0.75 // Controls how much effect document length has

	// Calculate IDF
	idf := calc.calculateIDF(term)

	// Get document and calculate its length
	doc, exists := calc.documentStore.Docs[docID]
	if !exists {
		return 0.0
	}

	docLength := calc.getDocumentLength(doc, searchableFields)
	avgDocLength := calc.getAverageDocumentLength(searchableFields)

	// Calculate BM25 TF component
	tf := termFreq
	bm25TF := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(float64(docLength)/avgDocLength)))

	return idf * bm25TF
}

// getAverageDocumentLength calculates the average document length across all documents
// This is used for BM25 calculation
func (calc *BM25Calculator) getAverageDocumentLength(searchableFields []string) float64 {
	if len(calc.documentStore.Docs) == 0 {
		return 0.0
	}

	totalLength := 0
	docCount := 0

	for _, doc := range calc.documentStore.Docs {
		docLength := calc.getDocumentLength(doc, searchableFields)
		totalLength += docLength
		docCount++
	}

	if docCount == 0 {
		return 0.0
	}

	return float64(totalLength) / float64(docCount)
}

// getDocumentLength calculates the total number of terms in a document across searchable fields
func (calc *BM25Calculator) getDocumentLength(doc map[string]interface{}, searchableFields []string) int {
	totalLength := 0

	for _, fieldName := range searchableFields {
		if fieldValue, exists := doc[fieldName]; exists {
			fieldLength := calc.getFieldLength(fieldValue)
			totalLength += fieldLength
		}
	}

	return totalLength
}

// getFieldLength calculates the number of terms in a field value
func (calc *BM25Calculator) getFieldLength(fieldValue interface{}) int {
	switch v := fieldValue.(type) {
	case string:
		// Simple word count approximation (split by spaces)
		if v == "" {
			return 0
		}
		// Count words by splitting on whitespace
		words := 0
		inWord := false
		for _, char := range v {
			if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
				inWord = false
			} else if !inWord {
				words++
				inWord = true
			}
		}
		return words
	case []string:
		totalWords := 0
		for _, str := range v {
			totalWords += calc.getFieldLength(str)
		}
		return totalWords
	case []interface{}:
		totalWords := 0
		for _, item := range v {
			if str, ok := item.(string); ok {
				totalWords += calc.getFieldLength(str)
			}
		}
		return totalWords
	default:
		return 0
	}
}

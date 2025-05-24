package index

// PostingEntry represents a document that contains a term, the field it appeared in,
// and a score related to its relevance in that field for that document.
type PostingEntry struct {
	DocID      uint32  // Internal numeric ID for efficiency
	FieldName  string  // The name of the field where the term was found (e.g., "title", "tags")
	Score      float64 // For now, term frequency within this field for this document
	IsFullWord bool    // True if this token represents a complete word from the original text, false if it's a generated n-gram (prefix)
	Positions  []int   // Added to store token positions
}

// PostingList is a slice of PostingEntry.
// Sorting strategy might depend on how it's used (e.g., by DocID then FieldName, or by Score).
// For indexing, it was sorted by Score (TF) descending to quickly find most relevant docs for a term.
// With FieldName, this might still apply if we consider TF within a field.
type PostingList []PostingEntry

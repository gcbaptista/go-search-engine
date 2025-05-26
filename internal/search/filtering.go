package search

import (
	"github.com/gcbaptista/go-search-engine/model"
)

// filterDocumentFields returns a new document containing only the specified fields.
// If retrievableFields are empty, returns the full document.
// The documentID field is always included regardless of the retrievableFields parameter.
func (s *Service) filterDocumentFields(doc model.Document, retrievableFields []string) model.Document {
	if len(retrievableFields) == 0 {
		return doc
	}

	filteredDoc := make(model.Document)

	// Always include documentID if it exists
	if docID, ok := doc["documentID"]; ok {
		filteredDoc["documentID"] = docID
	}

	// Add allowed fields to filteredDoc
	allowedFields := make(map[string]bool)
	for _, field := range retrievableFields {
		allowedFields[field] = true
	}

	for key, value := range doc {
		if allowedFields[key] {
			filteredDoc[key] = value
		}
	}

	return filteredDoc
}

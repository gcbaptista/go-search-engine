package search

import (
	"strconv"

	"github.com/gcbaptista/go-search-engine/model"
)

// filterDocumentFields returns a new document containing only the specified fields.
// If retrivableFields is empty, returns the full document.
// The documentID field is always included regardless of the retrivableFields parameter.
func (s *Service) filterDocumentFields(doc model.Document, retrivableFields []string) model.Document {
	if len(retrivableFields) == 0 {
		return doc
	}

	filteredDoc := make(model.Document)

	// Always include documentID if it exists
	if docID, ok := doc["documentID"]; ok {
		filteredDoc["documentID"] = docID
	}

	// Add allowed fields to filteredDoc
	allowedFields := make(map[string]bool)
	for _, field := range retrivableFields {
		allowedFields[field] = true
	}

	for key, value := range doc {
		if allowedFields[key] {
			filteredDoc[key] = value
		}
	}

	return filteredDoc
}

// deduplicateCandidates removes duplicate documents based on the specified field.
// It keeps the first occurrence (highest scoring) of each unique field value.
func (s *Service) deduplicateCandidates(hits []*candidateHit, distinctField string) []*candidateHit {
	if distinctField == "" || len(hits) == 0 {
		return hits
	}

	seen := make(map[string]bool)
	deduplicated := make([]*candidateHit, 0, len(hits))

	for _, hit := range hits {
		// Get the value of the distinct field from the document
		fieldValue, exists := hit.doc[distinctField]
		if !exists {
			// If the field doesn't exist, include the document (can't deduplicate)
			deduplicated = append(deduplicated, hit)
			continue
		}

		// Convert field value to string for comparison
		var fieldStr string
		switch v := fieldValue.(type) {
		case string:
			fieldStr = v
		case float64:
			fieldStr = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			fieldStr = strconv.Itoa(v)
		case bool:
			fieldStr = strconv.FormatBool(v)
		default:
			fieldStr = ""
		}

		if !seen[fieldStr] {
			seen[fieldStr] = true
			deduplicated = append(deduplicated, hit)
		}
	}

	return deduplicated
}

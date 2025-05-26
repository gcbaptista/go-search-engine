package search

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gcbaptista/go-search-engine/model"
)

// applyFilters applies filters to a list of candidate documents
func (s *Service) applyFilters(candidatesByDocID map[uint32]*candidateHit, queryFilters map[string]interface{}) map[uint32]*candidateHit {
	if len(queryFilters) == 0 {
		return candidatesByDocID
	}

	filteredCandidates := make(map[uint32]*candidateHit)

	for docID, candidate := range candidatesByDocID {
		if s.docMatchesFilters(candidate.doc, queryFilters) {
			filteredCandidates[docID] = candidate
		}
	}

	return filteredCandidates
}

// docMatchesFilters checks if a document matches a set of filters
func (s *Service) docMatchesFilters(doc model.Document, queryFilters map[string]interface{}) bool {
	filterableFieldsMap := make(map[string]struct{})
	for _, field := range s.settings.FilterableFields {
		filterableFieldsMap[field] = struct{}{}
	}

	for filterKey, filterValue := range queryFilters {
		fieldName, operator := parseFilterKey(filterKey)

		// Check if the field is filterable
		if _, isFilterable := filterableFieldsMap[fieldName]; !isFilterable {
			log.Printf("Warning: Field '%s' is not configured as filterable in index settings. Skipping filter.", fieldName)
			continue
		}

		docFieldVal, exists := doc[fieldName]
		if !exists {
			// Document doesn't have this field, so it doesn't match the filter
			return false
		}

		if !applyFilterLogic(docFieldVal, operator, filterValue, fieldName, s.settings.Name) {
			return false
		}
	}

	return true
}

// parseFilterKey parses a filter key to extract field name and operator
func parseFilterKey(key string) (string, string) {
	// Check for known operators in the key (order matters - check longer operators first)
	knownOperators := []string{
		"_contains_any_of",
		"_ncontains",
		"_contains",
		"_exact",
		"_gte",
		"_lte",
		"_gt",
		"_lt",
		"_ne",
		"_op", // Custom operator for testing
	}

	for _, op := range knownOperators {
		if strings.HasSuffix(key, op) {
			return strings.TrimSuffix(key, op), op
		}
	}

	// Default to equality (no operator)
	return key, ""
}

// applyFilterLogic applies the filter logic based on the operator
func applyFilterLogic(docFieldVal interface{}, operator string, filterValue interface{}, fieldNameForDebug, indexNameForDebug string) bool {
	switch operator {
	case "", "_exact":
		return applyEqualityFilter(docFieldVal, filterValue)
	case "_ne":
		return !applyEqualityFilter(docFieldVal, filterValue)
	case "_gt":
		return applyComparisonFilter(docFieldVal, filterValue, "gt")
	case "_gte":
		return applyComparisonFilter(docFieldVal, filterValue, "gte")
	case "_lt":
		return applyComparisonFilter(docFieldVal, filterValue, "lt")
	case "_lte":
		return applyComparisonFilter(docFieldVal, filterValue, "lte")
	case "_contains":
		return applyContainsFilter(docFieldVal, filterValue)
	case "_ncontains":
		return !applyContainsFilter(docFieldVal, filterValue)
	case "_contains_any_of":
		return applyContainsAnyOfFilter(docFieldVal, filterValue)
	default:
		log.Printf("Warning: Unknown filter operator '%s' for field '%s' in index '%s'. Treating as equality.", operator, fieldNameForDebug, indexNameForDebug)
		return applyEqualityFilter(docFieldVal, filterValue)
	}
}

// applyEqualityFilter checks if two values are equal
func applyEqualityFilter(docFieldVal, filterValue interface{}) bool {
	// Handle array fields
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if compareValues(item, filterValue) {
				return true
			}
		}
		return false
	}

	return compareValues(docFieldVal, filterValue)
}

// applyComparisonFilter applies comparison operators (gt, gte, lt, lte)
func applyComparisonFilter(docFieldVal, filterValue interface{}, operator string) bool {
	// Handle array fields - check if any element satisfies the condition
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if compareValuesWithOperator(item, filterValue, operator) {
				return true
			}
		}
		return false
	}

	return compareValuesWithOperator(docFieldVal, filterValue, operator)
}

// applyContainsFilter checks if a field contains a value
func applyContainsFilter(docFieldVal, filterValue interface{}) bool {
	// Handle array fields - check if any element contains the filter value
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if itemStr, isStr := item.(string); isStr {
				if filterStr, isFilterStr := filterValue.(string); isFilterStr {
					if strings.Contains(strings.ToLower(itemStr), strings.ToLower(filterStr)) {
						return true
					}
				}
			}
		}
		return false
	}

	// Handle string array fields
	if docStrArray, isStrArray := docFieldVal.([]string); isStrArray {
		for _, item := range docStrArray {
			if filterStr, isFilterStr := filterValue.(string); isFilterStr {
				if strings.Contains(strings.ToLower(item), strings.ToLower(filterStr)) {
					return true
				}
			}
		}
		return false
	}

	// Handle single string field
	if docStr, isDocStr := docFieldVal.(string); isDocStr {
		if filterStr, isFilterStr := filterValue.(string); isFilterStr {
			return strings.Contains(strings.ToLower(docStr), strings.ToLower(filterStr))
		}
	}

	return false
}

// applyContainsAnyOfFilter checks if a field contains any of the provided values
func applyContainsAnyOfFilter(docFieldVal, filterValue interface{}) bool {
	// Filter value should be an array
	filterArray, isFilterArray := filterValue.([]interface{})
	if !isFilterArray {
		return false
	}

	// Handle array fields - check if any element matches any filter value
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, docItem := range docArray {
			for _, filterItem := range filterArray {
				if compareValues(docItem, filterItem) {
					return true
				}
			}
		}
		return false
	}

	// Handle string array fields
	if docStrArray, isStrArray := docFieldVal.([]string); isStrArray {
		for _, docItem := range docStrArray {
			for _, filterItem := range filterArray {
				if compareValues(docItem, filterItem) {
					return true
				}
			}
		}
		return false
	}

	// Handle single field - check if it matches any filter value
	for _, filterItem := range filterArray {
		if compareValues(docFieldVal, filterItem) {
			return true
		}
	}

	return false
}

// compareValues compares two values for equality
func compareValues(docVal, filterVal interface{}) bool {
	// Direct equality check
	if docVal == filterVal {
		return true
	}

	// String comparison (case-sensitive)
	if docStr, isDocStr := docVal.(string); isDocStr {
		if filterStr, isFilterStr := filterVal.(string); isFilterStr {
			return docStr == filterStr
		}
	}

	// Numeric comparison
	if docFloat, docOk := convertToFloat64(docVal); docOk {
		if filterFloat, filterOk := convertToFloat64(filterVal); filterOk {
			return docFloat == filterFloat
		}
	}

	// Time comparison
	if docTime, docOk := convertToTime(docVal); docOk {
		if filterTime, filterOk := convertToTime(filterVal); filterOk {
			return docTime.Equal(filterTime)
		}
	}

	return false
}

// compareValuesWithOperator compares two values with a specific operator
func compareValuesWithOperator(docVal, filterVal interface{}, operator string) bool {
	// Numeric comparison
	if docFloat, docOk := convertToFloat64(docVal); docOk {
		if filterFloat, filterOk := convertToFloat64(filterVal); filterOk {
			switch operator {
			case "gt":
				return docFloat > filterFloat
			case "gte":
				return docFloat >= filterFloat
			case "lt":
				return docFloat < filterFloat
			case "lte":
				return docFloat <= filterFloat
			}
		}
	}

	// Time comparison
	if docTime, docOk := convertToTime(docVal); docOk {
		if filterTime, filterOk := convertToTime(filterVal); filterOk {
			switch operator {
			case "gt":
				return docTime.After(filterTime)
			case "gte":
				return docTime.After(filterTime) || docTime.Equal(filterTime)
			case "lt":
				return docTime.Before(filterTime)
			case "lte":
				return docTime.Before(filterTime) || docTime.Equal(filterTime)
			}
		}
	}

	// String comparison
	if docStr, isDocStr := docVal.(string); isDocStr {
		if filterStr, isFilterStr := filterVal.(string); isFilterStr {
			switch operator {
			case "gt":
				return docStr > filterStr
			case "gte":
				return docStr >= filterStr
			case "lt":
				return docStr < filterStr
			case "lte":
				return docStr <= filterStr
			}
		}
	}

	return false
}

// convertToFloat64 converts various numeric types to float64
func convertToFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// convertToTime converts various time representations to time.Time
func convertToTime(val interface{}) (time.Time, bool) {
	switch v := val.(type) {
	case time.Time:
		return v, true
	case string:
		// Try different time formats
		formats := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, true
			}
		}
	case int64:
		// Unix timestamp
		return time.Unix(v, 0), true
	case float64:
		// Unix timestamp as float
		return time.Unix(int64(v), 0), true
	}
	return time.Time{}, false
}

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

// Package config provides configuration structures for the search engine.
// It defines index settings, ranking criteria, and other configuration options.
package config

import (
	"strings"
)

// RankingCriterion defines a single field and direction to use for ranking search results.
// The ranking is applied in the order specified in the IndexSettings.RankingCriteria slice.
// Fields can be any document field, not just those in SearchableFields or FilterableFields.
type RankingCriterion struct {
	Field string `json:"field"` // Field name to rank by (e.g., "popularity", "title", "created_at"). Can be any document field.
	Order string `json:"order"` // Sort order: "asc" for ascending, "desc" for descending
}

// IndexSettings contains all configuration options for a search index.
// This includes which fields are searchable, filterable, ranking criteria,
// and typo tolerance settings.
//
// IMPORTANT: SearchableFields order matters for search priority!
// The search engine will:
// 1. Search the first field with exact matches
// 2. Search the first field with typo tolerance (if enabled for that field)
// 3. Only then proceed to the second field with exact matches
// 4. Search the second field with typo tolerance (if enabled)
// 5. Continue this pattern for all remaining fields
//
// This ensures higher-priority fields (like "title") are fully exhausted
// before moving to lower-priority fields (like "description").
type IndexSettings struct {
	Name                      string             `json:"name"`                         // Unique name for the index
	SearchableFields          []string           `json:"searchable_fields"`            // Fields that can be searched, in priority order (e.g., ["title", "cast", "genres"])
	FilterableFields          []string           `json:"filterable_fields"`            // Fields that can be used in filters (exact match, range)
	RankingCriteria           []RankingCriterion `json:"ranking_criteria"`             // Ordered list of ranking criteria, applied in sequence. Fields can be any document field.
	MinWordSizeFor1Typo       int                `json:"min_word_size_for_1_typo"`     // Minimum word length to allow 1 typo (e.g., 4)
	MinWordSizeFor2Typos      int                `json:"min_word_size_for_2_typos"`    // Minimum word length to allow 2 typos (e.g., 7)
	FieldsWithoutPrefixSearch []string           `json:"fields_without_prefix_search"` // Fields for which prefix/n-gram search is disabled (only whole words indexed). Must be in SearchableFields.
	NoTypoToleranceFields     []string           `json:"no_typo_tolerance_fields"`     // Fields for which typo tolerance is disabled (only exact matches). Must be in SearchableFields.
	NonTypoTolerantWords      []string           `json:"non_typo_tolerant_words"`      // Specific words that should never be typo-matched (e.g., sensitive terms, proper nouns)
	DistinctField             string             `json:"distinct_field"`               // Field to use for deduplication to avoid returning duplicate documents. Can be any document field.
	// Future: Field weights for relevance scoring
}

// ValidateFieldNames validates field names for basic requirements.
// Note: Field names ending with filter operators (like _exact, _gte) are now allowed
// since the current filter implementation uses explicit field/operator structures.
func (settings *IndexSettings) ValidateFieldNames() []string {
	var conflicts []string

	// Check for duplicate field names within each category
	conflicts = append(conflicts, checkDuplicates("searchable_fields", settings.SearchableFields)...)
	conflicts = append(conflicts, checkDuplicates("filterable_fields", settings.FilterableFields)...)
	conflicts = append(conflicts, checkDuplicates("fields_without_prefix_search", settings.FieldsWithoutPrefixSearch)...)
	conflicts = append(conflicts, checkDuplicates("no_typo_tolerance_fields", settings.NoTypoToleranceFields)...)
	conflicts = append(conflicts, checkDuplicates("non_typo_tolerant_words", settings.NonTypoTolerantWords)...)

	// Validate field references across configurations
	conflicts = append(conflicts, settings.validateFieldReferences()...)

	// Basic field name validation (empty names, etc.)
	allFields := make([]string, 0)
	allFields = append(allFields, settings.SearchableFields...)
	allFields = append(allFields, settings.FilterableFields...)
	allFields = append(allFields, settings.FieldsWithoutPrefixSearch...)
	allFields = append(allFields, settings.NoTypoToleranceFields...)
	allFields = append(allFields, settings.NonTypoTolerantWords...)
	if settings.DistinctField != "" {
		allFields = append(allFields, settings.DistinctField)
	}

	for _, field := range allFields {
		if strings.TrimSpace(field) == "" {
			conflicts = append(conflicts, "Field name cannot be empty or whitespace-only")
		}
	}

	return conflicts
}

// checkDuplicates checks for duplicate values in a slice and returns error messages
func checkDuplicates(fieldName string, fields []string) []string {
	var errors []string
	seen := make(map[string]bool)

	for _, field := range fields {
		if seen[field] {
			errors = append(errors, "Duplicate field '"+field+"' found in "+fieldName)
		}
		seen[field] = true
	}

	return errors
}

// validateFieldReferences validates that field references across configurations are valid
func (settings *IndexSettings) validateFieldReferences() []string {
	var errors []string

	// Create a set of all searchable fields for quick lookup
	searchableFieldsSet := make(map[string]bool)
	for _, field := range settings.SearchableFields {
		searchableFieldsSet[field] = true
	}

	// Create a set of all filterable fields for quick lookup
	filterableFieldsSet := make(map[string]bool)
	for _, field := range settings.FilterableFields {
		filterableFieldsSet[field] = true
	}

	// Validate that fields in FieldsWithoutPrefixSearch are actually searchable
	for _, field := range settings.FieldsWithoutPrefixSearch {
		if !searchableFieldsSet[field] {
			errors = append(errors, "Field '"+field+"' in fields_without_prefix_search is not in searchable_fields")
		}
	}

	// Validate that fields in NoTypoToleranceFields are actually searchable
	for _, field := range settings.NoTypoToleranceFields {
		if !searchableFieldsSet[field] {
			errors = append(errors, "Field '"+field+"' in no_typo_tolerance_fields is not in searchable_fields")
		}
	}

	// Note: DistinctField can be any field that exists in documents - no validation needed
	// Note: RankingCriteria fields can be any field that exists in documents - no validation needed

	// Validate ranking criteria order values only
	for _, criterion := range settings.RankingCriteria {
		// Validate order values
		if criterion.Order != "asc" && criterion.Order != "desc" {
			errors = append(errors, "Invalid order '"+criterion.Order+"' for field '"+criterion.Field+"' in ranking_criteria (must be 'asc' or 'desc')")
		}
	}

	return errors
}

// ApplyDefaults applies default values to the index settings
func (settings *IndexSettings) ApplyDefaults() {
	// Set default typo tolerance settings if not specified
	if settings.MinWordSizeFor1Typo == 0 {
		settings.MinWordSizeFor1Typo = 4
	}
	if settings.MinWordSizeFor2Typos == 0 {
		settings.MinWordSizeFor2Typos = 7
	}

	// Ensure MinWordSizeFor2Typos is at least as large as MinWordSizeFor1Typo
	if settings.MinWordSizeFor2Typos < settings.MinWordSizeFor1Typo {
		settings.MinWordSizeFor2Typos = settings.MinWordSizeFor1Typo + 1
	}

	// Initialize empty slices if nil to prevent nil pointer issues
	if settings.SearchableFields == nil {
		settings.SearchableFields = []string{}
	}
	if settings.FilterableFields == nil {
		settings.FilterableFields = []string{}
	}
	if settings.FieldsWithoutPrefixSearch == nil {
		settings.FieldsWithoutPrefixSearch = []string{}
	}
	if settings.NoTypoToleranceFields == nil {
		settings.NoTypoToleranceFields = []string{}
	}
	if settings.NonTypoTolerantWords == nil {
		settings.NonTypoTolerantWords = []string{}
	}
	if settings.RankingCriteria == nil {
		settings.RankingCriteria = []RankingCriterion{}
	}
}

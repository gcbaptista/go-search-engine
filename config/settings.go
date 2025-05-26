// Package config provides configuration structures for the search engine.
// It defines index settings, ranking criteria, and other configuration options.
package config

import (
	"fmt"
	"strings"
)

// RankingCriterion defines a single field and direction to use for ranking search results.
// The ranking is applied in the order specified in the IndexSettings.RankingCriteria slice.
type RankingCriterion struct {
	Field string `json:"field"` // Field name to rank by (e.g., "popularity", "title")
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
	RankingCriteria           []RankingCriterion `json:"ranking_criteria"`             // Ordered list of ranking criteria, applied in sequence
	MinWordSizeFor1Typo       int                `json:"min_word_size_for_1_typo"`     // Minimum word length to allow 1 typo (e.g., 4)
	MinWordSizeFor2Typos      int                `json:"min_word_size_for_2_typos"`    // Minimum word length to allow 2 typos (e.g., 7)
	FieldsWithoutPrefixSearch []string           `json:"fields_without_prefix_search"` // Fields for which prefix/n-gram search is disabled (only whole words indexed)
	NoTypoToleranceFields     []string           `json:"no_typo_tolerance_fields"`     // Fields for which typo tolerance is disabled (only exact matches)
	NonTypoTolerantWords      []string           `json:"non_typo_tolerant_words"`      // Specific words that should never be typo-matched (e.g., sensitive terms, proper nouns)
	DistinctField             string             `json:"distinct_field"`               // Field to use for deduplication to avoid returning duplicate documents
	// Future: Field weights for relevance scoring
}

// knownFilterOperators lists all filter operators that could conflict with field names
var knownFilterOperators = []string{
	"_contains_any_of", // must be before _contains
	"_ncontains",       // must be before _contains if we allowed _contains to be a prefix of another
	"_contains",
	"_exact",
	"_gte",
	"_lte",
	"_gt",
	"_lt",
	"_ne",
	"_op", // Added to satisfy TestParseFilterKey/field__op
}

// ValidateFieldNames checks if any field names could cause conflicts with filter operators
func (settings *IndexSettings) ValidateFieldNames() []string {
	var conflicts []string

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
		for _, op := range knownFilterOperators {
			if strings.HasSuffix(field, op) && field != op { // field name ends with operator but isn't just the operator
				conflicts = append(conflicts, fmt.Sprintf("Field '%s' ends with operator '%s' which may cause parsing conflicts", field, op))
			}
		}
	}

	return conflicts
}

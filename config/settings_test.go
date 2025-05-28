package config

import (
	"testing"
)

func TestValidateFieldReferences_RelaxedValidation(t *testing.T) {
	tests := []struct {
		name           string
		settings       IndexSettings
		expectedErrors int
		description    string
	}{
		{
			name: "ranking criteria can reference any field",
			settings: IndexSettings{
				Name:             "test_index",
				SearchableFields: []string{"title", "content"},
				FilterableFields: []string{"category", "year"},
				RankingCriteria: []RankingCriterion{
					{Field: "popularity", Order: "desc"}, // popularity is not in filterable fields - should be OK
					{Field: "rating", Order: "asc"},      // rating is not in filterable fields - should be OK
				},
			},
			expectedErrors: 0,
			description:    "Ranking criteria fields should not be required to be in filterable fields",
		},
		{
			name: "distinct field can be any field",
			settings: IndexSettings{
				Name:             "test_index",
				SearchableFields: []string{"title", "content"},
				FilterableFields: []string{"category", "year"},
				DistinctField:    "uuid", // uuid is not in searchable or filterable fields - should be OK
			},
			expectedErrors: 0,
			description:    "Distinct field should not be required to be in searchable or filterable fields",
		},
		{
			name: "special ranking criteria fields work",
			settings: IndexSettings{
				Name:             "test_index",
				SearchableFields: []string{"title", "content"},
				FilterableFields: []string{"category", "year"},
				RankingCriteria: []RankingCriterion{
					{Field: "~score", Order: "desc"},  // special field - should be OK
					{Field: "~filters", Order: "asc"}, // special field - should be OK
				},
			},
			expectedErrors: 0,
			description:    "Special ranking criteria fields (starting with ~) should work",
		},
		{
			name: "invalid ranking order still fails",
			settings: IndexSettings{
				Name:             "test_index",
				SearchableFields: []string{"title", "content"},
				FilterableFields: []string{"category", "year"},
				RankingCriteria: []RankingCriterion{
					{Field: "popularity", Order: "invalid"}, // invalid order - should fail
				},
			},
			expectedErrors: 1,
			description:    "Invalid ranking order should still be caught",
		},
		{
			name: "field reference validation still works for other fields",
			settings: IndexSettings{
				Name:                      "test_index",
				SearchableFields:          []string{"title", "content"},
				FilterableFields:          []string{"category", "year"},
				FieldsWithoutPrefixSearch: []string{"invalid_field"}, // not in searchable fields - should fail
			},
			expectedErrors: 1,
			description:    "Other field reference validations should still work",
		},
		{
			name: "comprehensive valid configuration",
			settings: IndexSettings{
				Name:             "test_index",
				SearchableFields: []string{"title", "content", "description"},
				FilterableFields: []string{"category", "year", "status"},
				RankingCriteria: []RankingCriterion{
					{Field: "popularity", Order: "desc"}, // any field OK
					{Field: "created_at", Order: "asc"},  // any field OK
					{Field: "~score", Order: "desc"},     // special field OK
				},
				DistinctField:             "uuid",                  // any field OK
				FieldsWithoutPrefixSearch: []string{"title"},       // must be in searchable fields
				NoTypoToleranceFields:     []string{"description"}, // must be in searchable fields
			},
			expectedErrors: 0,
			description:    "Comprehensive valid configuration should pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults to ensure consistent validation
			tt.settings.ApplyDefaults()

			errors := tt.settings.ValidateFieldNames()

			if len(errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d. Errors: %v", tt.expectedErrors, len(errors), errors)
				t.Logf("Description: %s", tt.description)
			}

			// Log the errors for debugging if any
			if len(errors) > 0 {
				t.Logf("Validation errors: %v", errors)
			}
		})
	}
}

func TestValidateFieldReferences_BackwardCompatibility(t *testing.T) {
	// Test that existing valid configurations still work
	settings := IndexSettings{
		Name:             "test_index",
		SearchableFields: []string{"title", "content"},
		FilterableFields: []string{"category", "year", "popularity"},
		RankingCriteria: []RankingCriterion{
			{Field: "popularity", Order: "desc"}, // in filterable fields
		},
		DistinctField: "category", // in filterable fields
	}

	settings.ApplyDefaults()
	errors := settings.ValidateFieldNames()

	if len(errors) != 0 {
		t.Errorf("Expected no errors for backward compatible configuration, got: %v", errors)
	}
}

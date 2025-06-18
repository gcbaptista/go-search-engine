package rules

import (
	"strings"
	"testing"

	"github.com/gcbaptista/go-search-engine/model"
)

func TestValidateRule_TypeSafeOperators(t *testing.T) {
	store := NewMemoryRuleStore()

	tests := []struct {
		name           string
		rule           model.Rule
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid query condition with string operator",
			rule: model.Rule{
				Name:      "Valid Query Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "query",
						Operator: "contains",
						Value:    "test string",
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "equals",
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid query condition with numeric operator - lte",
			rule: model.Rule{
				Name:      "Invalid Query Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "query",
						Operator: "lte", // This should fail!
						Value:    "test string",
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "equals",
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "operator 'lte' is not valid for query conditions. Valid operators: equals, contains, starts_with, ends_with",
		},
		{
			name: "invalid query condition with numeric operator - gt",
			rule: model.Rule{
				Name:      "Invalid Query Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "query",
						Operator: "gt", // This should fail!
						Value:    "test string",
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "equals",
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "operator 'gt' is not valid for query conditions. Valid operators: equals, contains, starts_with, ends_with",
		},
		{
			name: "valid result_count condition with numeric operator",
			rule: model.Rule{
				Name:      "Valid Result Count Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "result_count",
						Operator: "lte",
						Value:    10,
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "hide",
						Target: model.RuleTarget{
							Type:     "all_results",
							Operator: "equals",
							Value:    "",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid result_count condition with string operator",
			rule: model.Rule{
				Name:      "Invalid Result Count Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "result_count",
						Operator: "contains", // This should fail!
						Value:    10,
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "hide",
						Target: model.RuleTarget{
							Type:     "all_results",
							Operator: "equals",
							Value:    "",
						},
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "operator 'contains' is not valid for result_count conditions. Valid operators: equals, gt, gte, lt, lte",
		},
		{
			name: "invalid query condition with non-string value",
			rule: model.Rule{
				Name:      "Invalid Query Value Type Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "query",
						Operator: "equals",
						Value:    123, // This should fail - numbers not allowed for query
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "equals",
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "value must be a string for query conditions",
		},
		{
			name: "invalid action target with numeric operator",
			rule: model.Rule{
				Name:      "Invalid Action Target Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     "query",
						Operator: "equals",
						Value:    "test",
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "gte", // This should fail - numeric operator on string target
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "invalid target operator 'gte'. Valid operators for document_id: equals, contains, starts_with, ends_with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.validateRule(tt.rule)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error but got none")
					return
				}

				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain '%s', but got: %s", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestValidateRule_ComprehensiveOperatorValidation(t *testing.T) {
	store := NewMemoryRuleStore()

	// Test all invalid combinations systematically
	invalidCombinations := []struct {
		conditionType string
		operator      string
		value         interface{}
		description   string
	}{
		// Query conditions with invalid operators
		{"query", "gt", "test", "query with numeric operator gt"},
		{"query", "gte", "test", "query with numeric operator gte"},
		{"query", "lt", "test", "query with numeric operator lt"},
		{"query", "lte", "test", "query with numeric operator lte"},
		{"query", "in", "test", "query with collection operator in"},

		// Result count conditions with invalid operators
		{"result_count", "contains", 10, "result_count with string operator contains"},
		{"result_count", "starts_with", 10, "result_count with string operator starts_with"},
		{"result_count", "ends_with", 10, "result_count with string operator ends_with"},
		{"result_count", "in", 10, "result_count with collection operator in"},
	}

	for _, combo := range invalidCombinations {
		t.Run(combo.description, func(t *testing.T) {
			rule := model.Rule{
				Name:      "Test Rule",
				IndexName: "test",
				Conditions: []model.RuleCondition{
					{
						Type:     combo.conditionType,
						Operator: combo.operator,
						Value:    combo.value,
					},
				},
				Actions: []model.RuleAction{
					{
						Type: "pin",
						Target: model.RuleTarget{
							Type:     "document_id",
							Operator: "equals",
							Value:    "doc123",
						},
						Position: intPtr(1),
					},
				},
			}

			err := store.validateRule(rule)
			if err == nil {
				t.Errorf("Expected validation error for %s but got none", combo.description)
			}

			// Verify the error message contains information about valid operators
			if !strings.Contains(err.Error(), "Valid operators:") {
				t.Errorf("Error message should contain 'Valid operators:' but got: %s", err.Error())
			}
		})
	}
}
 
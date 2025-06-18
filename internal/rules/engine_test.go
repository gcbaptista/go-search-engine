package rules

import (
	"testing"

	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

func TestRuleEngine_PinAction(t *testing.T) {
	// Create a rule store and engine
	store := NewMemoryRuleStore()
	engine := NewEngine(store)

	// Create a test rule that pins document "2" to position 1 when searching for "matrix"
	rule := model.Rule{
		Name:      "Pin The Matrix for matrix searches",
		IndexName: "movies",
		IsActive:  true,
		Priority:  100,
		Conditions: []model.RuleCondition{
			{
				Type:     "query",
				Operator: "contains",
				Value:    "matrix",
			},
		},
		Actions: []model.RuleAction{
			{
				Type:     "pin",
				Position: intPtr(1),
				Target: model.RuleTarget{
					Type:     "document_id",
					Operator: "equals",
					Value:    "2",
				},
			},
		},
	}

	// Add rule to store
	_, err := store.CreateRule(rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create test search results
	results := []services.HitResult{
		{
			Document: model.Document{
				"documentID": "1",
				"title":      "Matrix Reloaded",
			},
			Score: 10.0,
		},
		{
			Document: model.Document{
				"documentID": "2",
				"title":      "The Matrix",
			},
			Score: 8.0,
		},
		{
			Document: model.Document{
				"documentID": "3",
				"title":      "Matrix Revolutions",
			},
			Score: 9.0,
		},
	}

	// Create evaluation context
	context := model.RuleEvaluationContext{
		Query:       "matrix movie",
		IndexName:   "movies",
		ResultCount: len(results),
	}

	// Apply rules
	modifiedResults, executionResult, err := engine.ApplyRules("movies", "matrix movie", results, context)
	if err != nil {
		t.Fatalf("Failed to apply rules: %v", err)
	}

	// Verify rule was applied
	if !executionResult.ModificationsApplied {
		t.Error("Expected modifications to be applied")
	}

	if len(executionResult.RulesApplied) != 1 {
		t.Errorf("Expected 1 rule to be applied, got %d", len(executionResult.RulesApplied))
	}

	// Verify document "2" is now at position 1 (index 0)
	if len(modifiedResults) != 3 {
		t.Errorf("Expected 3 results, got %d", len(modifiedResults))
	}

	firstResult := modifiedResults[0]
	if firstResult.Document["documentID"] != "2" {
		t.Errorf("Expected document '2' at position 1, got '%v'", firstResult.Document["documentID"])
	}
}

func TestRuleEngine_HideAction(t *testing.T) {
	// Create a rule store and engine
	store := NewMemoryRuleStore()
	engine := NewEngine(store)

	// Create a test rule that hides document "2" for family searches
	rule := model.Rule{
		Name:      "Hide specific movie for family searches",
		IndexName: "movies",
		IsActive:  true,
		Priority:  100,
		Conditions: []model.RuleCondition{
			{
				Type:     "query",
				Operator: "contains",
				Value:    "family",
			},
		},
		Actions: []model.RuleAction{
			{
				Type: "hide",
				Target: model.RuleTarget{
					Type:     "document_id",
					Operator: "equals",
					Value:    "2",
				},
			},
		},
	}

	// Add rule to store
	_, err := store.CreateRule(rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create test search results
	results := []services.HitResult{
		{
			Document: model.Document{
				"documentID": "1",
				"title":      "Family Movie",
				"rating":     "PG",
			},
			Score: 10.0,
		},
		{
			Document: model.Document{
				"documentID": "2",
				"title":      "Adult Movie",
				"rating":     "R",
			},
			Score: 9.0,
		},
		{
			Document: model.Document{
				"documentID": "3",
				"title":      "Kids Movie",
				"rating":     "G",
			},
			Score: 8.0,
		},
	}

	// Create evaluation context
	context := model.RuleEvaluationContext{
		Query:       "family movie",
		IndexName:   "movies",
		ResultCount: len(results),
	}

	// Apply rules
	modifiedResults, executionResult, err := engine.ApplyRules("movies", "family movie", results, context)
	if err != nil {
		t.Fatalf("Failed to apply rules: %v", err)
	}

	// Verify rule was applied
	if !executionResult.ModificationsApplied {
		t.Error("Expected modifications to be applied")
	}

	// Verify document "2" was hidden
	if len(modifiedResults) != 2 {
		t.Errorf("Expected 2 results after hiding document '2', got %d", len(modifiedResults))
	}

	// Verify document "2" is not in results
	for _, result := range modifiedResults {
		if result.Document["documentID"] == "2" {
			t.Error("Document '2' should have been hidden")
		}
	}
}

func TestEvaluateCondition(t *testing.T) {
	store := NewMemoryRuleStore()
	engine := NewEngine(store)

	// Create some test documents
	testResults := []services.HitResult{
		{
			Document: map[string]interface{}{
				"documentID": "doc1",
				"title":      "The Matrix",
				"year":       1999,
			},
		},
		{
			Document: map[string]interface{}{
				"documentID": "doc2",
				"title":      "The Matrix Reloaded",
				"year":       2003,
			},
		},
	}

	tests := []struct {
		name      string
		condition model.RuleCondition
		context   model.RuleEvaluationContext
		results   []services.HitResult
		expected  bool
	}{
		{
			name: "query contains - match",
			condition: model.RuleCondition{
				Type:     "query",
				Operator: "contains",
				Value:    "matrix",
			},
			context: model.RuleEvaluationContext{
				Query: "the matrix",
			},
			results:  testResults,
			expected: true,
		},
		{
			name: "query contains - no match",
			condition: model.RuleCondition{
				Type:     "query",
				Operator: "contains",
				Value:    "star wars",
			},
			context: model.RuleEvaluationContext{
				Query: "the matrix",
			},
			results:  testResults,
			expected: false,
		},
		{
			name: "query equals - match",
			condition: model.RuleCondition{
				Type:     "query",
				Operator: "equals",
				Value:    "the matrix",
			},
			context: model.RuleEvaluationContext{
				Query: "the matrix",
			},
			results:  testResults,
			expected: true,
		},
		{
			name: "query equals - no match",
			condition: model.RuleCondition{
				Type:     "query",
				Operator: "equals",
				Value:    "matrix",
			},
			context: model.RuleEvaluationContext{
				Query: "the matrix",
			},
			results:  testResults,
			expected: false,
		},
		{
			name: "result_count - match",
			condition: model.RuleCondition{
				Type:     "result_count",
				Operator: "equals",
				Value:    2,
			},
			context: model.RuleEvaluationContext{
				ResultCount: 2,
			},
			results:  testResults,
			expected: true,
		},
		{
			name: "result_count - no match",
			condition: model.RuleCondition{
				Type:     "result_count",
				Operator: "gt",
				Value:    5,
			},
			context: model.RuleEvaluationContext{
				ResultCount: 2,
			},
			results:  testResults,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.evaluateCondition(tt.condition, tt.context, tt.results)
			if result != tt.expected {
				t.Errorf("evaluateCondition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

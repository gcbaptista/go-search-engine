package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// Engine handles rule evaluation and application
type Engine struct {
	ruleStore RuleStore
}

// GetRule retrieves a specific rule by ID
func (e *Engine) GetRule(ruleID string) (model.Rule, error) {
	return e.ruleStore.GetRule(ruleID)
}

// CreateRule creates a new rule
func (e *Engine) CreateRule(rule model.Rule) (model.Rule, error) {
	return e.ruleStore.CreateRule(rule)
}

// UpdateRule updates an existing rule
func (e *Engine) UpdateRule(rule model.Rule) error {
	return e.ruleStore.UpdateRule(rule)
}

// DeleteRule deletes a rule
func (e *Engine) DeleteRule(ruleID string) error {
	return e.ruleStore.DeleteRule(ruleID)
}

// ListRules lists all rules with optional filtering
func (e *Engine) ListRules(indexName string, isActive *bool) ([]model.Rule, error) {
	return e.ruleStore.ListRules(indexName, isActive)
}

// RuleStore interface for rule persistence
type RuleStore interface {
	GetRulesByIndex(indexName string) ([]model.Rule, error)
	GetRule(ruleID string) (model.Rule, error)
	CreateRule(rule model.Rule) (model.Rule, error)
	UpdateRule(rule model.Rule) error
	DeleteRule(ruleID string) error
	ListRules(indexName string, isActive *bool) ([]model.Rule, error)
}

// NewEngine creates a new rule engine
func NewEngine(ruleStore RuleStore) *Engine {
	return &Engine{
		ruleStore: ruleStore,
	}
}

// ApplyRules evaluates and applies rules to search results
func (e *Engine) ApplyRules(indexName string, query string, results []services.HitResult, context model.RuleEvaluationContext) ([]services.HitResult, model.RuleExecutionResult, error) {
	startTime := time.Now()

	// Get applicable rules for this index
	rules, err := e.ruleStore.GetRulesByIndex(indexName)
	if err != nil {
		return results, model.RuleExecutionResult{}, fmt.Errorf("failed to get rules: %w", err)
	}

	// Also get global rules (index_name = "*")
	globalRules, err := e.ruleStore.GetRulesByIndex("*")
	if err != nil {
		return results, model.RuleExecutionResult{}, fmt.Errorf("failed to get global rules: %w", err)
	}

	// Combine and filter active rules
	allRules := append(rules, globalRules...)
	var activeRules []model.Rule
	for _, rule := range allRules {
		if rule.IsActive {
			activeRules = append(activeRules, rule)
		}
	}

	// Sort rules by priority (higher priority first)
	sort.Slice(activeRules, func(i, j int) bool {
		return activeRules[i].Priority > activeRules[j].Priority
	})

	// Set context values
	context.Query = query
	context.IndexName = indexName
	context.ResultCount = len(results)

	executionResult := model.RuleExecutionResult{
		RulesEvaluated:       make([]string, 0),
		RulesApplied:         make([]model.RuleApplication, 0),
		ExecutionTimeMs:      0,
		ModificationsApplied: false,
	}

	modifiedResults := make([]services.HitResult, len(results))
	copy(modifiedResults, results)

	// Track pinned positions to avoid conflicts
	pinnedPositions := make(map[int]bool)

	// Evaluate and apply each rule
	for _, rule := range activeRules {
		executionResult.RulesEvaluated = append(executionResult.RulesEvaluated, rule.ID)

		// Check if rule conditions are met
		if e.evaluateConditions(rule.Conditions, context, modifiedResults) {
			// Apply rule actions
			applied, affected, newResults := e.applyActions(rule.Actions, modifiedResults, pinnedPositions)
			if applied {
				modifiedResults = newResults
				executionResult.ModificationsApplied = true

				ruleApp := model.RuleApplication{
					RuleID:            rule.ID,
					RuleName:          rule.Name,
					ActionsApplied:    []string{},
					DocumentsAffected: affected,
					AppliedAt:         time.Now(),
				}

				for _, action := range rule.Actions {
					ruleApp.ActionsApplied = append(ruleApp.ActionsApplied, action.Type)
				}

				executionResult.RulesApplied = append(executionResult.RulesApplied, ruleApp)
			}
		}
	}

	executionResult.ExecutionTimeMs = float64(time.Since(startTime).Nanoseconds()) / 1e6

	return modifiedResults, executionResult, nil
}

// evaluateConditions checks if all conditions in a rule are met
func (e *Engine) evaluateConditions(conditions []model.RuleCondition, context model.RuleEvaluationContext, results []services.HitResult) bool {
	for _, condition := range conditions {
		if !e.evaluateCondition(condition, context, results) {
			return false
		}
	}
	return true
}

// evaluateCondition checks if a single condition is met
func (e *Engine) evaluateCondition(condition model.RuleCondition, context model.RuleEvaluationContext, results []services.HitResult) bool {
	switch condition.Type {
	case "query":
		return e.evaluateStringCondition(context.Query, condition.Operator, condition.Value)
	case "result_count":
		return e.evaluateNumericCondition(float64(context.ResultCount), condition.Operator, condition.Value)
	default:
		return false
	}
}

// evaluateStringCondition evaluates string-based conditions (always case-insensitive)
func (e *Engine) evaluateStringCondition(actual string, operator string, expected interface{}) bool {
	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}

	// Always case-insensitive for simplicity
	actual = strings.ToLower(actual)
	expectedStr = strings.ToLower(expectedStr)

	switch operator {
	case "equals":
		return actual == expectedStr
	case "contains":
		return strings.Contains(actual, expectedStr)
	case "starts_with":
		return strings.HasPrefix(actual, expectedStr)
	case "ends_with":
		return strings.HasSuffix(actual, expectedStr)
	default:
		return false
	}
}

// evaluateNumericCondition evaluates numeric conditions
func (e *Engine) evaluateNumericCondition(actual float64, operator string, expected interface{}) bool {
	var expectedFloat float64

	switch v := expected.(type) {
	case float64:
		expectedFloat = v
	case int:
		expectedFloat = float64(v)
	case string:
		var err error
		expectedFloat, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
	default:
		return false
	}

	switch operator {
	case "equals":
		return actual == expectedFloat
	case "gt":
		return actual > expectedFloat
	case "gte":
		return actual >= expectedFloat
	case "lt":
		return actual < expectedFloat
	case "lte":
		return actual <= expectedFloat
	default:
		return false
	}
}

// evaluateFieldCondition evaluates conditions on document fields
func (e *Engine) evaluateFieldCondition(fieldValue interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "equals", "contains", "starts_with", "ends_with":
		if fieldStr, ok := fieldValue.(string); ok {
			return e.evaluateStringCondition(fieldStr, operator, expected)
		}
		return fieldValue == expected
	case "gt", "gte", "lt", "lte":
		if fieldFloat, ok := convertToFloat(fieldValue); ok {
			return e.evaluateNumericCondition(fieldFloat, operator, expected)
		}
		return false
	case "in":
		if expectedSlice, ok := expected.([]interface{}); ok {
			for _, item := range expectedSlice {
				if fieldValue == item {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// convertToFloat converts various numeric types to float64
func convertToFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// applyActions applies rule actions to the results
func (e *Engine) applyActions(actions []model.RuleAction, results []services.HitResult, pinnedPositions map[int]bool) (bool, int, []services.HitResult) {
	modified := false
	totalAffected := 0
	modifiedResults := make([]services.HitResult, len(results))
	copy(modifiedResults, results)

	for _, action := range actions {
		actionModified, affected, newResults := e.applyAction(action, modifiedResults, pinnedPositions)
		if actionModified {
			modified = true
			totalAffected += affected
			modifiedResults = newResults
		}
	}

	return modified, totalAffected, modifiedResults
}

// applyAction applies a single action to the results
func (e *Engine) applyAction(action model.RuleAction, results []services.HitResult, pinnedPositions map[int]bool) (bool, int, []services.HitResult) {
	switch action.Type {
	case "pin":
		return e.applyPinAction(action, results, pinnedPositions)
	case "hide":
		return e.applyHideAction(action, results)
	default:
		return false, 0, results
	}
}

// applyPinAction pins matching documents to a specific position
func (e *Engine) applyPinAction(action model.RuleAction, results []services.HitResult, pinnedPositions map[int]bool) (bool, int, []services.HitResult) {
	if action.Position == nil || *action.Position < 1 || *action.Position > len(results) {
		return false, 0, results
	}

	position := *action.Position - 1 // Convert to 0-based index

	// Check if position is already pinned
	if pinnedPositions[position] {
		return false, 0, results
	}

	// Find matching documents
	var matchingIndices []int
	for i, result := range results {
		if e.matchesTarget(action.Target, result) {
			matchingIndices = append(matchingIndices, i)
		}
	}

	if len(matchingIndices) == 0 {
		return false, 0, results
	}

	// Pin the first matching document to the specified position
	modifiedResults := make([]services.HitResult, len(results))
	copy(modifiedResults, results)

	// Remove the document from its current position
	matchingDoc := modifiedResults[matchingIndices[0]]
	modifiedResults = append(modifiedResults[:matchingIndices[0]], modifiedResults[matchingIndices[0]+1:]...)

	// Insert at the specified position
	modifiedResults = append(modifiedResults[:position], append([]services.HitResult{matchingDoc}, modifiedResults[position:]...)...)

	// Mark position as pinned
	pinnedPositions[position] = true

	return true, 1, modifiedResults
}

// applyHideAction removes matching documents from results
func (e *Engine) applyHideAction(action model.RuleAction, results []services.HitResult) (bool, int, []services.HitResult) {
	var modifiedResults []services.HitResult
	affected := 0

	for _, result := range results {
		if !e.matchesTarget(action.Target, result) {
			modifiedResults = append(modifiedResults, result)
		} else {
			affected++
		}
	}

	return affected > 0, affected, modifiedResults
}

// matchesTarget checks if a document matches a rule target
func (e *Engine) matchesTarget(target model.RuleTarget, result services.HitResult) bool {
	switch target.Type {
	case "all_results":
		return true
	case "document_id":
		// Try multiple possible document ID field names
		var docID interface{}
		var exists bool

		// Try "documentID" first (as per Document.GetDocumentID())
		if docID, exists = result.Document["documentID"]; !exists {
			// Try "id" as fallback
			if docID, exists = result.Document["id"]; !exists {
				// Try "_id" as another fallback (common in some systems)
				docID, exists = result.Document["_id"]
			}
		}

		if exists {
			return e.evaluateFieldCondition(docID, target.Operator, target.Value)
		}
		return false
	default:
		return false
	}
}

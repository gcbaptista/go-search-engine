package model

import (
	"time"
)

// RuleCondition represents a condition that must be met for a rule to be applied
type RuleCondition struct {
	Type     string      `json:"type"`     // "query_contains", "query_exact", "result_count"
	Operator string      `json:"operator"` // "equals", "contains", "starts_with", "ends_with", "gt", "gte", "lt", "lte", "in"
	Value    interface{} `json:"value"`    // The value to compare against
}

// RuleAction represents an action to be performed when rule conditions are met
type RuleAction struct {
	Type     string     `json:"type"`               // "pin", "hide"
	Target   RuleTarget `json:"target"`             // What to target for the action
	Position *int       `json:"position,omitempty"` // For pin actions, 1-based position
}

// RuleTarget specifies how to identify documents to apply actions to
type RuleTarget struct {
	Type     string      `json:"type"`     // "document_id", "all_results"
	Operator string      `json:"operator"` // "equals", "contains", "starts_with", "ends_with", "gt", "gte", "lt", "lte", "in"
	Value    interface{} `json:"value"`    // The value to match
}

// Rule represents a complete search rule with conditions and actions
type Rule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	IndexName   string          `json:"index_name"` // Index this rule applies to, "*" for all indexes
	IsActive    bool            `json:"is_active"`
	Priority    int             `json:"priority"`   // Higher numbers = higher priority when multiple rules match
	Conditions  []RuleCondition `json:"conditions"` // All conditions must be met (AND logic)
	Actions     []RuleAction    `json:"actions"`    // All actions will be applied if conditions match
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   string          `json:"created_by,omitempty"`
}

// RuleEvaluationContext contains the context needed to evaluate rules
type RuleEvaluationContext struct {
	Query       string `json:"query"`
	IndexName   string `json:"index_name"`
	ResultCount int    `json:"result_count"`
}

// RuleApplication represents the result of applying a rule
type RuleApplication struct {
	RuleID            string    `json:"rule_id"`
	RuleName          string    `json:"rule_name"`
	ActionsApplied    []string  `json:"actions_applied"`
	DocumentsAffected int       `json:"documents_affected"`
	AppliedAt         time.Time `json:"applied_at"`
}

// RuleExecutionResult contains the results of rule engine execution
type RuleExecutionResult struct {
	RulesEvaluated       []string          `json:"rules_evaluated"`
	RulesApplied         []RuleApplication `json:"rules_applied"`
	ExecutionTimeMs      float64           `json:"execution_time_ms"`
	ModificationsApplied bool              `json:"modifications_applied"`
}

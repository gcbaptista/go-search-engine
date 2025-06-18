package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gcbaptista/go-search-engine/model"
	"github.com/google/uuid"
)

// MemoryRuleStore is an in-memory implementation of the RuleStore interface
type MemoryRuleStore struct {
	rules map[string]model.Rule
	mutex sync.RWMutex
}

// NewMemoryRuleStore creates a new in-memory rule store
func NewMemoryRuleStore() *MemoryRuleStore {
	return &MemoryRuleStore{
		rules: make(map[string]model.Rule),
	}
}

// GetRulesByIndex retrieves all rules for a specific index
func (s *MemoryRuleStore) GetRulesByIndex(indexName string) ([]model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var rules []model.Rule
	for _, rule := range s.rules {
		if rule.IndexName == indexName || rule.IndexName == "*" {
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

// GetRule retrieves a specific rule by ID
func (s *MemoryRuleStore) GetRule(ruleID string) (model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rule, exists := s.rules[ruleID]
	if !exists {
		return model.Rule{}, fmt.Errorf("rule with ID %s not found", ruleID)
	}

	return rule, nil
}

// CreateRule creates a new rule
func (s *MemoryRuleStore) CreateRule(rule model.Rule) (model.Rule, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Generate ID if not provided
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// Check if rule with this ID already exists
	if _, exists := s.rules[rule.ID]; exists {
		return model.Rule{}, fmt.Errorf("rule with ID %s already exists", rule.ID)
	}

	// Set timestamps
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return model.Rule{}, fmt.Errorf("invalid rule: %w", err)
	}

	s.rules[rule.ID] = rule
	return rule, nil
}

// UpdateRule updates an existing rule
func (s *MemoryRuleStore) UpdateRule(rule model.Rule) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if rule exists
	existing, exists := s.rules[rule.ID]
	if !exists {
		return fmt.Errorf("rule with ID %s not found", rule.ID)
	}

	// Preserve creation timestamp
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	s.rules[rule.ID] = rule
	return nil
}

// DeleteRule deletes a rule
func (s *MemoryRuleStore) DeleteRule(ruleID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.rules[ruleID]; !exists {
		return fmt.Errorf("rule with ID %s not found", ruleID)
	}

	delete(s.rules, ruleID)
	return nil
}

// ListRules lists all rules with optional filtering
func (s *MemoryRuleStore) ListRules(indexName string, isActive *bool) ([]model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var rules []model.Rule
	for _, rule := range s.rules {
		// Filter by index name if specified
		if indexName != "" && rule.IndexName != indexName && rule.IndexName != "*" {
			continue
		}

		// Filter by active status if specified
		if isActive != nil && rule.IsActive != *isActive {
			continue
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// validateRule validates a rule's structure and logic
func (s *MemoryRuleStore) validateRule(rule model.Rule) error {
	// Validate basic fields
	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if rule.IndexName == "" {
		return fmt.Errorf("index name cannot be empty")
	}

	if len(rule.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}

	if len(rule.Actions) == 0 {
		return fmt.Errorf("rule must have at least one action")
	}

	// Validate conditions
	validConditionTypes := map[string]bool{
		"query":        true,
		"result_count": true,
	}

	// Define operators by category for better type safety
	stringOperators := map[string]bool{
		"equals":      true,
		"contains":    true,
		"starts_with": true,
		"ends_with":   true,
	}

	numericOperators := map[string]bool{
		"equals": true,
		"gt":     true,
		"gte":    true,
		"lt":     true,
		"lte":    true,
	}

	for index, condition := range rule.Conditions {
		if !validConditionTypes[condition.Type] {
			return fmt.Errorf("condition %d: invalid condition type '%s'", index, condition.Type)
		}

		switch condition.Type {
		case "query":
			// For query conditions, only string operators are valid
			if !stringOperators[condition.Operator] {
				return fmt.Errorf("condition %d: operator '%s' is not valid for query conditions. Valid operators: equals, contains, starts_with, ends_with", index, condition.Operator)
			}

			// Value should be a string
			if _, ok := condition.Value.(string); !ok {
				return fmt.Errorf("condition %d: value must be a string for query conditions", index)
			}
		case "result_count":
			// For result_count, only numeric operators are valid
			if !numericOperators[condition.Operator] {
				return fmt.Errorf("condition %d: operator '%s' is not valid for result_count conditions. Valid operators: equals, gt, gte, lt, lte", index, condition.Operator)
			}

			// Value should be convertible to number
			switch condition.Value.(type) {
			case int, int64, float64:
				// Valid numeric types
			case string:
				// Try to parse as number
				if _, err := strconv.ParseFloat(condition.Value.(string), 64); err != nil {
					return fmt.Errorf("condition %d: value must be numeric for result_count", index)
				}
			default:
				return fmt.Errorf("condition %d: value must be numeric for result_count", index)
			}
		default:
			// For other types, value should not be nil
			if condition.Value == nil {
				return fmt.Errorf("condition %d: value cannot be nil", index)
			}
		}
	}

	// Validate actions
	validActionTypes := map[string]bool{
		"pin":  true,
		"hide": true,
	}

	validTargetTypes := map[string]bool{
		"document_id": true,
		"all_results": true,
	}

	for actionIndex, action := range rule.Actions {
		if !validActionTypes[action.Type] {
			return fmt.Errorf("action %d: invalid action type '%s'", actionIndex, action.Type)
		}

		if !validTargetTypes[action.Target.Type] {
			return fmt.Errorf("action %d: invalid target type '%s'", actionIndex, action.Target.Type)
		}

		// Validate target
		switch action.Target.Type {
		case "document_id":
			if action.Target.Value == nil {
				return fmt.Errorf("action %d: value cannot be nil for document_id target", actionIndex)
			}
		case "all_results":
			// No additional validation needed for all_results
		}

		// Validate position for pin actions
		if action.Type == "pin" {
			if action.Position == nil || *action.Position < 1 {
				return fmt.Errorf("action %d: pin actions must have a position >= 1", actionIndex)
			}
		}

		// Validate that non-pin actions don't have position
		if action.Type != "pin" && action.Position != nil {
			return fmt.Errorf("action %d: only pin actions can have a position", actionIndex)
		}

		// Validate target operators based on target type
		switch action.Target.Type {
		case "document_id":
			// Document IDs should use string operators
			if !stringOperators[action.Target.Operator] {
				return fmt.Errorf("action %d: invalid target operator '%s'. Valid operators for document_id: equals, contains, starts_with, ends_with", actionIndex, action.Target.Operator)
			}
		case "all_results":
			// all_results targets don't typically use operators, but if they do, any is acceptable
		}
	}

	return nil
}

// FileRuleStore is a file-based implementation of the RuleStore interface
type FileRuleStore struct {
	rules        map[string]model.Rule
	mutex        sync.RWMutex
	dataFilePath string
}

// NewFileRuleStore creates a new file-based rule store
func NewFileRuleStore(dataDir string) *FileRuleStore {
	store := &FileRuleStore{
		rules:        make(map[string]model.Rule),
		dataFilePath: filepath.Join(dataDir, "rules.json"),
	}

	// Load existing rules data
	if err := store.loadData(); err != nil {
		// If file doesn't exist, that's fine - we'll create it on first save
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: Failed to load rules data: %v\n", err)
		}
	}

	return store
}

// GetRulesByIndex retrieves all rules for a specific index
func (s *FileRuleStore) GetRulesByIndex(indexName string) ([]model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var rules []model.Rule
	for _, rule := range s.rules {
		if rule.IndexName == indexName || rule.IndexName == "*" {
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

// GetRule retrieves a specific rule by ID
func (s *FileRuleStore) GetRule(ruleID string) (model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rule, exists := s.rules[ruleID]
	if !exists {
		return model.Rule{}, fmt.Errorf("rule with ID %s not found", ruleID)
	}

	return rule, nil
}

// CreateRule creates a new rule
func (s *FileRuleStore) CreateRule(rule model.Rule) (model.Rule, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Generate ID if not provided
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// Check if rule with this ID already exists
	if _, exists := s.rules[rule.ID]; exists {
		return model.Rule{}, fmt.Errorf("rule with ID %s already exists", rule.ID)
	}

	// Set timestamps
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return model.Rule{}, fmt.Errorf("invalid rule: %w", err)
	}

	s.rules[rule.ID] = rule

	// Persist to disk
	if err := s.saveData(); err != nil {
		// Rollback the in-memory change
		delete(s.rules, rule.ID)
		return model.Rule{}, fmt.Errorf("failed to persist rule: %w", err)
	}

	return rule, nil
}

// UpdateRule updates an existing rule
func (s *FileRuleStore) UpdateRule(rule model.Rule) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if rule exists
	existing, exists := s.rules[rule.ID]
	if !exists {
		return fmt.Errorf("rule with ID %s not found", rule.ID)
	}

	// Preserve creation timestamp
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	// Store old rule for rollback
	oldRule := s.rules[rule.ID]
	s.rules[rule.ID] = rule

	// Persist to disk
	if err := s.saveData(); err != nil {
		// Rollback the in-memory change
		s.rules[rule.ID] = oldRule
		return fmt.Errorf("failed to persist rule update: %w", err)
	}

	return nil
}

// DeleteRule deletes a rule
func (s *FileRuleStore) DeleteRule(ruleID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	rule, exists := s.rules[ruleID]
	if !exists {
		return fmt.Errorf("rule with ID %s not found", ruleID)
	}

	delete(s.rules, ruleID)

	// Persist to disk
	if err := s.saveData(); err != nil {
		// Rollback the in-memory change
		s.rules[ruleID] = rule
		return fmt.Errorf("failed to persist rule deletion: %w", err)
	}

	return nil
}

// ListRules lists all rules with optional filtering
func (s *FileRuleStore) ListRules(indexName string, isActive *bool) ([]model.Rule, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var rules []model.Rule
	for _, rule := range s.rules {
		// Filter by index name if specified
		if indexName != "" && rule.IndexName != indexName && rule.IndexName != "*" {
			continue
		}

		// Filter by active status if specified
		if isActive != nil && rule.IsActive != *isActive {
			continue
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// loadData loads rules from the data file
func (s *FileRuleStore) loadData() error {
	// Ensure directory exists
	dir := filepath.Dir(s.dataFilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Read file
	data, err := os.ReadFile(s.dataFilePath)
	if err != nil {
		return err
	}

	// Parse JSON
	var rules []model.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("failed to parse rules data: %w", err)
	}

	// Load into memory map
	s.rules = make(map[string]model.Rule)
	for _, rule := range rules {
		s.rules[rule.ID] = rule
	}

	return nil
}

// saveData saves rules to the data file
func (s *FileRuleStore) saveData() error {
	// Convert map to slice
	var rules []model.Rule
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules data: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.dataFilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(s.dataFilePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write rules data: %w", err)
	}

	return nil
}

// validateRule validates a rule's structure and logic (shared with MemoryRuleStore)
func (s *FileRuleStore) validateRule(rule model.Rule) error {
	// Validate basic fields
	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if rule.IndexName == "" {
		return fmt.Errorf("index name cannot be empty")
	}

	if len(rule.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}

	if len(rule.Actions) == 0 {
		return fmt.Errorf("rule must have at least one action")
	}

	// Validate conditions
	validConditionTypes := map[string]bool{
		"query":        true,
		"result_count": true,
	}

	// Define operators by category for better type safety
	stringOperators := map[string]bool{
		"equals":      true,
		"contains":    true,
		"starts_with": true,
		"ends_with":   true,
	}

	numericOperators := map[string]bool{
		"equals": true,
		"gt":     true,
		"gte":    true,
		"lt":     true,
		"lte":    true,
	}

	for index, condition := range rule.Conditions {
		if !validConditionTypes[condition.Type] {
			return fmt.Errorf("condition %d: invalid condition type '%s'", index, condition.Type)
		}

		switch condition.Type {
		case "query":
			// For query conditions, only string operators are valid
			if !stringOperators[condition.Operator] {
				return fmt.Errorf("condition %d: operator '%s' is not valid for query conditions. Valid operators: equals, contains, starts_with, ends_with", index, condition.Operator)
			}

			// Value should be a string
			if _, ok := condition.Value.(string); !ok {
				return fmt.Errorf("condition %d: value must be a string for query conditions", index)
			}
		case "result_count":
			// For result_count, only numeric operators are valid
			if !numericOperators[condition.Operator] {
				return fmt.Errorf("condition %d: operator '%s' is not valid for result_count conditions. Valid operators: equals, gt, gte, lt, lte", index, condition.Operator)
			}

			// Value should be convertible to number
			switch condition.Value.(type) {
			case int, int64, float64:
				// Valid numeric types
			case string:
				// Try to parse as number
				if _, err := strconv.ParseFloat(condition.Value.(string), 64); err != nil {
					return fmt.Errorf("condition %d: result_count value must be numeric", index)
				}
			default:
				return fmt.Errorf("condition %d: result_count value must be numeric", index)
			}
		}
	}

	// Validate actions
	validActionTypes := map[string]bool{
		"pin":  true,
		"hide": true,
	}

	validTargetTypes := map[string]bool{
		"document_id": true,
		"all_results": true,
	}

	for index, action := range rule.Actions {
		if !validActionTypes[action.Type] {
			return fmt.Errorf("action %d: invalid action type '%s'", index, action.Type)
		}

		if !validTargetTypes[action.Target.Type] {
			return fmt.Errorf("action %d: invalid target type '%s'", index, action.Target.Type)
		}

		// Validate target operators (targets typically use string operators)
		if action.Target.Type == "document_id" && !stringOperators[action.Target.Operator] {
			return fmt.Errorf("action %d: invalid target operator '%s'. Valid operators for document_id: equals, contains, starts_with, ends_with", index, action.Target.Operator)
		}

		// Pin actions must have a position
		if action.Type == "pin" && action.Position == nil {
			return fmt.Errorf("action %d: pin actions must specify a position", index)
		}

		// Position must be positive if specified
		if action.Position != nil && *action.Position < 1 {
			return fmt.Errorf("action %d: position must be 1 or greater", index)
		}
	}

	return nil
}

package api

import (
	"net/http"
	"strconv"

	"github.com/gcbaptista/go-search-engine/internal/rules"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gin-gonic/gin"
)

// RuleRequest represents the JSON request for creating/updating rules
type RuleRequest struct {
	Name        string                `json:"name" binding:"required"`
	Description string                `json:"description,omitempty"`
	IndexName   string                `json:"index_name" binding:"required"`
	IsActive    bool                  `json:"is_active"`
	Priority    int                   `json:"priority"`
	Conditions  []model.RuleCondition `json:"conditions" binding:"required"`
	Actions     []model.RuleAction    `json:"actions" binding:"required"`
	CreatedBy   string                `json:"created_by,omitempty"`
}

// RuleResponse represents the JSON response for single rule operations
type RuleResponse struct {
	Status  string     `json:"status"`
	Rule    model.Rule `json:"rule"`
	Message string     `json:"message,omitempty"`
}

// RuleListResponse represents the JSON response for listing rules
type RuleListResponse struct {
	Status string       `json:"status"`
	Rules  []model.Rule `json:"rules"`
	Count  int          `json:"count"`
}

// RuleMessageResponse represents the JSON response for operations that only return a message
type RuleMessageResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreateRuleHandler handles POST /api/v1/rules
func (api *API) CreateRuleHandler(c *gin.Context) {
	var req RuleRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid request body: "+err.Error())
		return
	}

	// Convert request to rule model
	rule := model.Rule{
		Name:        req.Name,
		Description: req.Description,
		IndexName:   req.IndexName,
		IsActive:    req.IsActive,
		Priority:    req.Priority,
		Conditions:  req.Conditions,
		Actions:     req.Actions,
		CreatedBy:   req.CreatedBy,
	}

	// Create the rule and get the created rule with ID
	createdRule, err := api.ruleEngine.CreateRule(rule)
	if err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Failed to create rule: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, RuleResponse{
		Status:  "success",
		Rule:    createdRule,
		Message: "Rule created successfully",
	})
}

// GetRuleHandler handles GET /api/v1/rules/:ruleId
func (api *API) GetRuleHandler(c *gin.Context) {
	ruleID := c.Param("ruleId")
	if ruleID == "" {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Rule ID is required")
		return
	}

	rule, err := api.ruleEngine.GetRule(ruleID)
	if err != nil {
		SendError(c, http.StatusNotFound, ErrorCodeNotFound, "Rule not found: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, RuleResponse{
		Status: "success",
		Rule:   rule,
	})
}

// UpdateRuleHandler handles PUT /api/v1/rules/:ruleId
func (api *API) UpdateRuleHandler(c *gin.Context) {
	ruleID := c.Param("ruleId")
	if ruleID == "" {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Rule ID is required")
		return
	}

	var req RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid request body: "+err.Error())
		return
	}

	// Convert request to rule model
	rule := model.Rule{
		ID:          ruleID,
		Name:        req.Name,
		Description: req.Description,
		IndexName:   req.IndexName,
		IsActive:    req.IsActive,
		Priority:    req.Priority,
		Conditions:  req.Conditions,
		Actions:     req.Actions,
		CreatedBy:   req.CreatedBy,
	}

	// Update the rule
	if err := api.ruleEngine.UpdateRule(rule); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Failed to update rule: "+err.Error())
		return
	}

	// Get the updated rule
	updatedRule, err := api.ruleEngine.GetRule(ruleID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to retrieve updated rule: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, RuleResponse{
		Status:  "success",
		Rule:    updatedRule,
		Message: "Rule updated successfully",
	})
}

// DeleteRuleHandler handles DELETE /api/v1/rules/:ruleId
func (api *API) DeleteRuleHandler(c *gin.Context) {
	ruleID := c.Param("ruleId")
	if ruleID == "" {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Rule ID is required")
		return
	}

	// Check if rule exists first
	_, err := api.ruleEngine.GetRule(ruleID)
	if err != nil {
		SendError(c, http.StatusNotFound, ErrorCodeNotFound, "Rule not found: "+err.Error())
		return
	}

	// Delete the rule
	if err := api.ruleEngine.DeleteRule(ruleID); err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to delete rule: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, RuleMessageResponse{
		Status:  "success",
		Message: "Rule deleted successfully",
	})
}

// ListRulesHandler handles GET /api/v1/rules
func (api *API) ListRulesHandler(c *gin.Context) {
	// Get query parameters
	indexName := c.Query("index_name")
	isActiveStr := c.Query("is_active")

	var isActive *bool
	if isActiveStr != "" {
		if isActiveVal, err := strconv.ParseBool(isActiveStr); err == nil {
			isActive = &isActiveVal
		} else {
			SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid is_active parameter: must be true or false")
			return
		}
	}

	rules, err := api.ruleEngine.ListRules(indexName, isActive)
	if err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to list rules: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, RuleListResponse{
		Status: "success",
		Rules:  rules,
		Count:  len(rules),
	})
}

// ToggleRuleHandler handles POST /api/v1/rules/:ruleId/toggle
func (api *API) ToggleRuleHandler(c *gin.Context) {
	ruleID := c.Param("ruleId")
	if ruleID == "" {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Rule ID is required")
		return
	}

	// Get current rule
	rule, err := api.ruleEngine.GetRule(ruleID)
	if err != nil {
		SendError(c, http.StatusNotFound, ErrorCodeNotFound, "Rule not found: "+err.Error())
		return
	}

	// Toggle active status
	rule.IsActive = !rule.IsActive

	// Update the rule
	if err := api.ruleEngine.UpdateRule(rule); err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to toggle rule status: "+err.Error())
		return
	}

	// Get the updated rule
	updatedRule, err := api.ruleEngine.GetRule(ruleID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to retrieve updated rule: "+err.Error())
		return
	}

	status := "activated"
	if !updatedRule.IsActive {
		status = "deactivated"
	}

	c.JSON(http.StatusOK, RuleResponse{
		Status:  "success",
		Rule:    updatedRule,
		Message: "Rule " + status + " successfully",
	})
}

// TestRuleHandler handles POST /api/v1/rules/test
// This endpoint allows testing rule conditions without persisting the rule
func (api *API) TestRuleHandler(c *gin.Context) {
	var req struct {
		Rule    model.Rule                  `json:"rule" binding:"required"`
		Context model.RuleEvaluationContext `json:"context" binding:"required"`
		Results []services.HitResult        `json:"results" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid request body: "+err.Error())
		return
	}

	// Create a temporary rule engine with just this rule
	tempStore := rules.NewMemoryRuleStore()
	tempEngine := rules.NewEngine(tempStore)

	// Add the test rule to the temporary store
	_, err := tempStore.CreateRule(req.Rule)
	if err != nil {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidQuery, "Invalid rule: "+err.Error())
		return
	}

	// Apply the rule to the test results
	modifiedResults, executionResult, err := tempEngine.ApplyRules(
		req.Context.IndexName,
		req.Context.Query,
		req.Results,
		req.Context,
	)
	if err != nil {
		SendError(c, http.StatusInternalServerError, ErrorCodeInternalServerError, "Failed to test rule: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"test_result": gin.H{
			"original_results": req.Results,
			"modified_results": modifiedResults,
			"execution_result": executionResult,
			"rule_applied":     executionResult.ModificationsApplied,
		},
	})
}

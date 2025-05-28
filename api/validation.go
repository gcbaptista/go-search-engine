// Package api provides validation utilities for API request handling.
package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/model"
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult holds the result of validation operations
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// AddError adds a validation error to the result
func (vr *ValidationResult) AddError(field, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// ValidateIndexName validates an index name parameter
func ValidateIndexName(indexName string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if indexName == "" {
		result.AddError("indexName", "Index name is required")
		return result
	}

	if strings.TrimSpace(indexName) != indexName {
		result.AddError("indexName", "Index name cannot have leading or trailing whitespace")
		return result
	}

	return result
}

// ValidateDocumentID validates a document ID
func ValidateDocumentID(documentID string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if documentID == "" {
		result.AddError("documentID", "Document ID is required")
		return result
	}

	if strings.TrimSpace(documentID) != documentID {
		result.AddError("documentID", "Document ID cannot have leading or trailing whitespace")
		return result
	}

	return result
}

// ValidateIndexSettings validates index settings for creation
func ValidateIndexSettings(settings *config.IndexSettings) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if settings == nil {
		result.AddError("settings", "Index settings are required")
		return result
	}

	if settings.Name == "" {
		result.AddError("name", "Index name is required")
	}

	// Apply defaults before validation
	settings.ApplyDefaults()

	// Validate field names and references
	if conflicts := settings.ValidateFieldNames(); len(conflicts) > 0 {
		for _, conflict := range conflicts {
			result.AddError("field_validation", conflict)
		}
	}

	return result
}

// ValidateDocuments validates a slice of documents for addition
func ValidateDocuments(docs []model.Document) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if len(docs) == 0 {
		result.AddError("documents", "No documents provided")
		return result
	}

	for i, doc := range docs {
		// Validate documentID presence and format
		docIDVal, exists := doc["documentID"]
		if !exists {
			result.AddError(fmt.Sprintf("documents[%d].documentID", i), "Document must have a 'documentID' field")
			continue
		}

		docIDStr, ok := docIDVal.(string)
		if !ok {
			result.AddError(fmt.Sprintf("documents[%d].documentID", i), "Document ID must be a string")
			continue
		}

		if strings.TrimSpace(docIDStr) == "" {
			result.AddError(fmt.Sprintf("documents[%d].documentID", i), "Document ID cannot be empty or whitespace-only")
			continue
		}
	}

	return result
}

// ValidatePagination validates pagination parameters
func ValidatePagination(page, pageSize int) (int, int, *ValidationResult) {
	result := &ValidationResult{Valid: true}

	// Set defaults
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// Validate limits
	if pageSize > 100 {
		pageSize = 100 // Maximum page size
	}

	if page < 1 {
		result.AddError("page", "Page number must be greater than 0")
	}

	if pageSize < 1 {
		result.AddError("page_size", "Page size must be greater than 0")
	}

	return page, pageSize, result
}

// ValidateRenameRequest validates a rename index request
func ValidateRenameRequest(oldName, newName string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if oldName == "" {
		result.AddError("oldName", "Current index name is required")
	}

	if newName == "" {
		result.AddError("new_name", "New name is required and cannot be empty")
	}

	if strings.TrimSpace(newName) != newName {
		result.AddError("new_name", "New name cannot have leading or trailing whitespace")
	}

	if oldName == newName {
		result.AddError("new_name", "New name must be different from current name")
	}

	return result
}

// SendValidationError sends a standardized validation error response
func SendValidationError(c *gin.Context, result *ValidationResult) {
	// Use the new structured error response format
	SendStructuredValidationError(c, result)
}

// ValidateJSONBinding validates JSON binding and returns a standardized error
func ValidateJSONBinding(c *gin.Context, target interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := c.ShouldBindJSON(target); err != nil {
		result.AddError("request_body", "Invalid request body: "+err.Error())
	}

	return result
}

// ValidateQueryBinding validates query parameter binding
func ValidateQueryBinding(c *gin.Context, target interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := c.ShouldBindQuery(target); err != nil {
		result.AddError("query_parameters", "Invalid query parameters: "+err.Error())
	}

	return result
}

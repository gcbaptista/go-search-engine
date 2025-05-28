package api

import (
	"testing"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/model"
)

func TestValidationResult_AddError(t *testing.T) {
	result := &ValidationResult{Valid: true}

	result.AddError("field1", "error message")

	if result.Valid {
		t.Error("Expected Valid to be false after adding error")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if result.Errors[0].Field != "field1" {
		t.Errorf("Expected field 'field1', got '%s'", result.Errors[0].Field)
	}

	if result.Errors[0].Message != "error message" {
		t.Errorf("Expected message 'error message', got '%s'", result.Errors[0].Message)
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	result := &ValidationResult{Valid: true}

	if result.HasErrors() {
		t.Error("Expected HasErrors to be false for empty result")
	}

	result.AddError("field", "message")

	if !result.HasErrors() {
		t.Error("Expected HasErrors to be true after adding error")
	}
}

func TestValidateIndexName(t *testing.T) {
	tests := []struct {
		name      string
		indexName string
		wantValid bool
		wantError string
	}{
		{
			name:      "valid index name",
			indexName: "test-index",
			wantValid: true,
		},
		{
			name:      "empty index name",
			indexName: "",
			wantValid: false,
			wantError: "Index name is required",
		},
		{
			name:      "index name with leading whitespace",
			indexName: " test-index",
			wantValid: false,
			wantError: "Index name cannot have leading or trailing whitespace",
		},
		{
			name:      "index name with trailing whitespace",
			indexName: "test-index ",
			wantValid: false,
			wantError: "Index name cannot have leading or trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateIndexName(tt.indexName)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateIndexName() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Errors) > 0 {
				if result.Errors[0].Message != tt.wantError {
					t.Errorf("ValidateIndexName() error = %v, want %v", result.Errors[0].Message, tt.wantError)
				}
			}
		})
	}
}

func TestValidateDocumentID(t *testing.T) {
	tests := []struct {
		name       string
		documentID string
		wantValid  bool
		wantError  string
	}{
		{
			name:       "valid document ID",
			documentID: "doc-123",
			wantValid:  true,
		},
		{
			name:       "empty document ID",
			documentID: "",
			wantValid:  false,
			wantError:  "Document ID is required",
		},
		{
			name:       "document ID with leading whitespace",
			documentID: " doc-123",
			wantValid:  false,
			wantError:  "Document ID cannot have leading or trailing whitespace",
		},
		{
			name:       "document ID with trailing whitespace",
			documentID: "doc-123 ",
			wantValid:  false,
			wantError:  "Document ID cannot have leading or trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDocumentID(tt.documentID)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateDocumentID() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Errors) > 0 {
				if result.Errors[0].Message != tt.wantError {
					t.Errorf("ValidateDocumentID() error = %v, want %v", result.Errors[0].Message, tt.wantError)
				}
			}
		})
	}
}

func TestValidateIndexSettings(t *testing.T) {
	tests := []struct {
		name      string
		settings  *config.IndexSettings
		wantValid bool
		wantError string
	}{
		{
			name: "valid settings",
			settings: &config.IndexSettings{
				Name:             "test-index",
				SearchableFields: []string{"title", "content"},
				FilterableFields: []string{"category"},
			},
			wantValid: true,
		},
		{
			name:      "nil settings",
			settings:  nil,
			wantValid: false,
			wantError: "Index settings are required",
		},
		{
			name: "empty name",
			settings: &config.IndexSettings{
				Name:             "",
				SearchableFields: []string{"title"},
			},
			wantValid: false,
			wantError: "Index name is required",
		},
		{
			name: "invalid field reference",
			settings: &config.IndexSettings{
				Name:                      "test-index",
				SearchableFields:          []string{"title"},
				FieldsWithoutPrefixSearch: []string{"content"}, // content not in searchable fields
			},
			wantValid: false,
			wantError: "Field 'content' in fields_without_prefix_search is not in searchable_fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateIndexSettings(tt.settings)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateIndexSettings() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if err.Message == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateIndexSettings() expected error '%v' not found in %v", tt.wantError, result.Errors)
				}
			}
		})
	}
}

func TestValidateDocuments(t *testing.T) {
	tests := []struct {
		name      string
		docs      []model.Document
		wantValid bool
		wantError string
	}{
		{
			name: "valid documents",
			docs: []model.Document{
				{"documentID": "doc1", "title": "Test"},
				{"documentID": "doc2", "title": "Test 2"},
			},
			wantValid: true,
		},
		{
			name:      "empty documents",
			docs:      []model.Document{},
			wantValid: false,
			wantError: "No documents provided",
		},
		{
			name: "missing documentID",
			docs: []model.Document{
				{"title": "Test"},
			},
			wantValid: false,
			wantError: "Document must have a 'documentID' field",
		},
		{
			name: "non-string documentID",
			docs: []model.Document{
				{"documentID": 123, "title": "Test"},
			},
			wantValid: false,
			wantError: "Document ID must be a string",
		},
		{
			name: "empty documentID",
			docs: []model.Document{
				{"documentID": "", "title": "Test"},
			},
			wantValid: false,
			wantError: "Document ID cannot be empty or whitespace-only",
		},
		{
			name: "whitespace-only documentID",
			docs: []model.Document{
				{"documentID": "   ", "title": "Test"},
			},
			wantValid: false,
			wantError: "Document ID cannot be empty or whitespace-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDocuments(tt.docs)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateDocuments() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if err.Message == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateDocuments() expected error '%v' not found in %v", tt.wantError, result.Errors)
				}
			}
		})
	}
}

func TestValidatePagination(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
		wantValid    bool
	}{
		{
			name:         "valid pagination",
			page:         2,
			pageSize:     20,
			wantPage:     2,
			wantPageSize: 20,
			wantValid:    true,
		},
		{
			name:         "zero page defaults to 1",
			page:         0,
			pageSize:     20,
			wantPage:     1,
			wantPageSize: 20,
			wantValid:    true,
		},
		{
			name:         "zero page size defaults to 10",
			page:         1,
			pageSize:     0,
			wantPage:     1,
			wantPageSize: 10,
			wantValid:    true,
		},
		{
			name:         "page size over 100 capped to 100",
			page:         1,
			pageSize:     150,
			wantPage:     1,
			wantPageSize: 100,
			wantValid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPageSize, result := ValidatePagination(tt.page, tt.pageSize)

			if gotPage != tt.wantPage {
				t.Errorf("ValidatePagination() page = %v, want %v", gotPage, tt.wantPage)
			}

			if gotPageSize != tt.wantPageSize {
				t.Errorf("ValidatePagination() pageSize = %v, want %v", gotPageSize, tt.wantPageSize)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("ValidatePagination() Valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestValidateRenameRequest(t *testing.T) {
	tests := []struct {
		name      string
		oldName   string
		newName   string
		wantValid bool
		wantError string
	}{
		{
			name:      "valid rename",
			oldName:   "old-index",
			newName:   "new-index",
			wantValid: true,
		},
		{
			name:      "empty old name",
			oldName:   "",
			newName:   "new-index",
			wantValid: false,
			wantError: "Current index name is required",
		},
		{
			name:      "empty new name",
			oldName:   "old-index",
			newName:   "",
			wantValid: false,
			wantError: "New name is required and cannot be empty",
		},
		{
			name:      "new name with whitespace",
			oldName:   "old-index",
			newName:   " new-index ",
			wantValid: false,
			wantError: "New name cannot have leading or trailing whitespace",
		},
		{
			name:      "same names",
			oldName:   "same-index",
			newName:   "same-index",
			wantValid: false,
			wantError: "New name must be different from current name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRenameRequest(tt.oldName, tt.newName)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateRenameRequest() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if err.Message == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateRenameRequest() expected error '%v' not found in %v", tt.wantError, result.Errors)
				}
			}
		})
	}
}

package errors

import (
	"errors"
	"testing"
)

func TestIndexNotFoundError(t *testing.T) {
	indexName := "test-index"
	err := NewIndexNotFoundError(indexName)

	// Test error message
	expectedMsg := "index named 'test-index' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrIndexNotFound) {
		t.Error("Expected error to match ErrIndexNotFound sentinel")
	}

	// Test that it doesn't match other sentinels
	if errors.Is(err, ErrDocumentNotFound) {
		t.Error("Error should not match ErrDocumentNotFound")
	}
}

func TestIndexAlreadyExistsError(t *testing.T) {
	indexName := "existing-index"
	err := NewIndexAlreadyExistsError(indexName)

	// Test error message
	expectedMsg := "index named 'existing-index' already exists"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrIndexAlreadyExists) {
		t.Error("Expected error to match ErrIndexAlreadyExists sentinel")
	}
}

func TestDocumentNotFoundError(t *testing.T) {
	// Test without index name
	docID := "doc123"
	err := NewDocumentNotFoundError(docID)

	expectedMsg := "document with ID 'doc123' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test with index name
	indexName := "test-index"
	err2 := NewDocumentNotFoundError(docID, indexName)

	expectedMsg2 := "document with ID 'doc123' not found in index 'test-index'"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg2, err2.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrDocumentNotFound) {
		t.Error("Expected error to match ErrDocumentNotFound sentinel")
	}
	if !errors.Is(err2, ErrDocumentNotFound) {
		t.Error("Expected error with index to match ErrDocumentNotFound sentinel")
	}
}

func TestJobNotFoundError(t *testing.T) {
	jobID := "job-456"
	err := NewJobNotFoundError(jobID)

	expectedMsg := "job with ID 'job-456' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrJobNotFound) {
		t.Error("Expected error to match ErrJobNotFound sentinel")
	}
}

func TestValidationError(t *testing.T) {
	// Test with field
	field := "name"
	message := "cannot be empty"
	err := NewValidationError(field, message)

	expectedMsg := "validation error for field 'name': cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test without field
	err2 := NewValidationError("", message)

	expectedMsg2 := "validation error: cannot be empty"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg2, err2.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrInvalidInput) {
		t.Error("Expected error to match ErrInvalidInput sentinel")
	}
	if !errors.Is(err2, ErrInvalidInput) {
		t.Error("Expected error without field to match ErrInvalidInput sentinel")
	}
}

func TestSameNameError(t *testing.T) {
	name := "same-name"
	err := NewSameNameError(name)

	expectedMsg := "new name 'same-name' is the same as the current name"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test Is() method
	if !errors.Is(err, ErrSameName) {
		t.Error("Expected error to match ErrSameName sentinel")
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that our custom errors can be wrapped and unwrapped
	originalErr := NewIndexNotFoundError("test-index")
	wrappedErr := errors.Join(originalErr, errors.New("additional context"))

	// Should still be able to detect the original error
	if !errors.Is(wrappedErr, ErrIndexNotFound) {
		t.Error("Expected wrapped error to still match ErrIndexNotFound sentinel")
	}

	// Should be able to unwrap to get the original error
	var indexErr *IndexNotFoundError
	if !errors.As(wrappedErr, &indexErr) {
		t.Error("Expected to be able to unwrap to IndexNotFoundError")
	}

	if indexErr.IndexName != "test-index" {
		t.Errorf("Expected index name 'test-index', got '%s'", indexErr.IndexName)
	}
}

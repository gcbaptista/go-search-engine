package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions
var (
	// ErrIndexNotFound is returned when an index is not found
	ErrIndexNotFound = errors.New("index not found")

	// ErrIndexAlreadyExists is returned when trying to create an index that already exists
	ErrIndexAlreadyExists = errors.New("index already exists")

	// ErrDocumentNotFound is returned when a document is not found
	ErrDocumentNotFound = errors.New("document not found")

	// ErrJobNotFound is returned when a job is not found
	ErrJobNotFound = errors.New("job not found")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrSameName is returned when trying to rename to the same name
	ErrSameName = errors.New("same name provided")
)

// IndexNotFoundError represents an index not found error with context
type IndexNotFoundError struct {
	IndexName string
}

func (e *IndexNotFoundError) Error() string {
	return fmt.Sprintf("index named '%s' not found", e.IndexName)
}

func (e *IndexNotFoundError) Is(target error) bool {
	return target == ErrIndexNotFound
}

// NewIndexNotFoundError creates a new IndexNotFoundError
func NewIndexNotFoundError(indexName string) *IndexNotFoundError {
	return &IndexNotFoundError{IndexName: indexName}
}

// IndexAlreadyExistsError represents an index already exists error with context
type IndexAlreadyExistsError struct {
	IndexName string
}

func (e *IndexAlreadyExistsError) Error() string {
	return fmt.Sprintf("index named '%s' already exists", e.IndexName)
}

func (e *IndexAlreadyExistsError) Is(target error) bool {
	return target == ErrIndexAlreadyExists
}

// NewIndexAlreadyExistsError creates a new IndexAlreadyExistsError
func NewIndexAlreadyExistsError(indexName string) *IndexAlreadyExistsError {
	return &IndexAlreadyExistsError{IndexName: indexName}
}

// DocumentNotFoundError represents a document not found error with context
type DocumentNotFoundError struct {
	DocumentID string
	IndexName  string
}

func (e *DocumentNotFoundError) Error() string {
	if e.IndexName != "" {
		return fmt.Sprintf("document with ID '%s' not found in index '%s'", e.DocumentID, e.IndexName)
	}
	return fmt.Sprintf("document with ID '%s' not found", e.DocumentID)
}

func (e *DocumentNotFoundError) Is(target error) bool {
	return target == ErrDocumentNotFound
}

// NewDocumentNotFoundError creates a new DocumentNotFoundError
func NewDocumentNotFoundError(documentID string, indexName ...string) *DocumentNotFoundError {
	err := &DocumentNotFoundError{DocumentID: documentID}
	if len(indexName) > 0 {
		err.IndexName = indexName[0]
	}
	return err
}

// JobNotFoundError represents a job not found error with context
type JobNotFoundError struct {
	JobID string
}

func (e *JobNotFoundError) Error() string {
	return fmt.Sprintf("job with ID '%s' not found", e.JobID)
}

func (e *JobNotFoundError) Is(target error) bool {
	return target == ErrJobNotFound
}

// NewJobNotFoundError creates a new JobNotFoundError
func NewJobNotFoundError(jobID string) *JobNotFoundError {
	return &JobNotFoundError{JobID: jobID}
}

// ValidationError represents an input validation error with context
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// SameNameError represents an error when trying to rename to the same name
type SameNameError struct {
	Name string
}

func (e *SameNameError) Error() string {
	return fmt.Sprintf("new name '%s' is the same as the current name", e.Name)
}

func (e *SameNameError) Is(target error) bool {
	return target == ErrSameName
}

// NewSameNameError creates a new SameNameError
func NewSameNameError(name string) *SameNameError {
	return &SameNameError{Name: name}
}

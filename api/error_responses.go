package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorCode represents standardized error codes for the API
type ErrorCode string

const (
	// Client Error Codes (4xx)
	ErrorCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	ErrorCodeIndexNotFound    ErrorCode = "INDEX_NOT_FOUND"
	ErrorCodeDocumentNotFound ErrorCode = "DOCUMENT_NOT_FOUND"
	ErrorCodeJobNotFound      ErrorCode = "JOB_NOT_FOUND"
	ErrorCodeIndexExists      ErrorCode = "INDEX_ALREADY_EXISTS"
	ErrorCodeInvalidRequest   ErrorCode = "INVALID_REQUEST"
	ErrorCodeInvalidJSON      ErrorCode = "INVALID_JSON"
	ErrorCodeInvalidQuery     ErrorCode = "INVALID_QUERY"
	ErrorCodeSameName         ErrorCode = "SAME_NAME_PROVIDED"

	// Server Error Codes (5xx)
	ErrorCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrorCodeIndexingFailed     ErrorCode = "INDEXING_FAILED"
	ErrorCodeSearchFailed       ErrorCode = "SEARCH_FAILED"
	ErrorCodePersistenceFailed  ErrorCode = "PERSISTENCE_FAILED"
	ErrorCodeJobExecutionFailed ErrorCode = "JOB_EXECUTION_FAILED"
)

// ErrorDetail provides additional context for an error
type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// APIError represents a standardized API error response
type APIError struct {
	Error     string        `json:"error"`
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	Details   []ErrorDetail `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	RequestID string        `json:"request_id,omitempty"`
}

// APIErrorResponse creates a standardized error response
func APIErrorResponse(code ErrorCode, message string, details ...ErrorDetail) *APIError {
	return &APIError{
		Error:     "Request failed",
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// SendError sends a standardized error response
func SendError(c *gin.Context, statusCode int, code ErrorCode, message string, details ...ErrorDetail) {
	errorResponse := APIErrorResponse(code, message, details...)

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			errorResponse.RequestID = id
		}
	}

	c.JSON(statusCode, errorResponse)
}

// SendStructuredValidationError sends a validation error with structured details using the new error format
func SendStructuredValidationError(c *gin.Context, result *ValidationResult) {
	details := make([]ErrorDetail, len(result.Errors))
	for i, err := range result.Errors {
		details[i] = ErrorDetail{
			Field:   err.Field,
			Message: err.Message,
			Code:    "VALIDATION_ERROR",
		}
	}

	SendError(c, http.StatusBadRequest, ErrorCodeValidationFailed, "Request validation failed", details...)
}

// SendIndexNotFoundError sends a standardized index not found error
func SendIndexNotFoundError(c *gin.Context, indexName string) {
	SendError(c, http.StatusNotFound, ErrorCodeIndexNotFound,
		"Index '"+indexName+"' not found")
}

// SendDocumentNotFoundError sends a standardized document not found error
func SendDocumentNotFoundError(c *gin.Context, documentID, indexName string) {
	message := "Document '" + documentID + "' not found"
	if indexName != "" {
		message += " in index '" + indexName + "'"
	}
	SendError(c, http.StatusNotFound, ErrorCodeDocumentNotFound, message)
}

// SendJobNotFoundError sends a standardized job not found error
func SendJobNotFoundError(c *gin.Context, jobID string) {
	SendError(c, http.StatusNotFound, ErrorCodeJobNotFound,
		"Job '"+jobID+"' not found")
}

// SendIndexExistsError sends a standardized index already exists error
func SendIndexExistsError(c *gin.Context, indexName string) {
	SendError(c, http.StatusConflict, ErrorCodeIndexExists,
		"Index '"+indexName+"' already exists")
}

// SendSameNameError sends a standardized same name error
func SendSameNameError(c *gin.Context, name string) {
	SendError(c, http.StatusBadRequest, ErrorCodeSameName,
		"New name '"+name+"' is the same as the current name")
}

// SendInvalidJSONError sends a standardized invalid JSON error
func SendInvalidJSONError(c *gin.Context, err error) {
	SendError(c, http.StatusBadRequest, ErrorCodeInvalidJSON,
		"Invalid JSON in request body: "+err.Error())
}

// SendInternalError sends a standardized internal server error
func SendInternalError(c *gin.Context, operation string, err error) {
	SendError(c, http.StatusInternalServerError, ErrorCodeInternalError,
		"Internal error during "+operation+": "+err.Error())
}

// SendIndexingError sends a standardized indexing error
func SendIndexingError(c *gin.Context, operation string, err error) {
	SendError(c, http.StatusInternalServerError, ErrorCodeIndexingFailed,
		"Indexing operation failed ("+operation+"): "+err.Error())
}

// SendSearchError sends a standardized search error
func SendSearchError(c *gin.Context, indexName string, err error) {
	SendError(c, http.StatusInternalServerError, ErrorCodeSearchFailed,
		"Search failed on index '"+indexName+"': "+err.Error())
}

// SendJobExecutionError sends a standardized job execution error
func SendJobExecutionError(c *gin.Context, operation string, err error) {
	SendError(c, http.StatusInternalServerError, ErrorCodeJobExecutionFailed,
		"Failed to start "+operation+" job: "+err.Error())
}

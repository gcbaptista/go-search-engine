package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/internal/engine"
	internalErrors "github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/model"
)

// AddDocumentsHandler handles adding/updating documents in an index.
func (api *API) AddDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	// Read the raw JSON data first
	var rawData interface{}
	if result := ValidateJSONBinding(c, &rawData); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	var docs []model.Document

	// Check if the raw data is a slice (array) or a single object
	if dataSlice, isSlice := rawData.([]interface{}); isSlice {
		// Handle array of documents
		docs = make([]model.Document, len(dataSlice))
		for i, item := range dataSlice {
			if docMap, isMap := item.(map[string]interface{}); isMap {
				docs[i] = docMap
			} else {
				SendError(c, http.StatusBadRequest, ErrorCodeInvalidRequest, fmt.Sprintf("Document at index %d is not a valid object", i))
				return
			}
		}
	} else if docMap, isMap := rawData.(map[string]interface{}); isMap {
		// Handle single document
		docs = []model.Document{docMap}
	} else {
		SendError(c, http.StatusBadRequest, ErrorCodeInvalidRequest, "Invalid request body. Expecting a document object or an array of documents")
		return
	}

	// Validate documents
	if result := ValidateDocuments(docs); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	// Clean up document IDs (trim whitespace)
	for i := range docs {
		docMap := docs[i]
		if docIDVal, exists := docMap["documentID"]; exists {
			if docIDStr, ok := docIDVal.(string); ok {
				docMap["documentID"] = strings.TrimSpace(docIDStr)
			}
		}
	}

	// Add documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.AddDocumentsAsync(indexName, docs)
		if err != nil {
			SendJobExecutionError(c, "document addition", err)
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":         "accepted",
			"message":        fmt.Sprintf("Document addition started for index '%s' (%d documents)", indexName, len(docs)),
			"job_id":         jobID,
			"document_count": len(docs),
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.AddDocuments(docs)
		if err != nil {
			SendIndexingError(c, "add documents", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d document(s) added/updated in index '%s'", len(docs), indexName)})
	}
}

// DeleteAllDocumentsHandler handles the request to delete all documents from an index.
func (api *API) DeleteAllDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	// Delete all documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteAllDocumentsAsync(indexName)
		if err != nil {
			SendJobExecutionError(c, "document deletion", err)
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "accepted",
			"message": fmt.Sprintf("Document deletion started for index '%s'", indexName),
			"job_id":  jobID,
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.DeleteAllDocuments()
		if err != nil {
			SendIndexingError(c, "delete all documents", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "All documents deleted from index '" + indexName + "'"})
	}
}

// DocumentListRequest defines the structure for document listing requests
type DocumentListRequest struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// GetDocumentsHandler lists documents in an index with pagination
func (api *API) GetDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	var req DocumentListRequest

	// Validate query binding
	if result := ValidateQueryBinding(c, &req); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	// Validate and set pagination defaults
	page, pageSize, result := ValidatePagination(req.Page, req.PageSize)
	if result.HasErrors() {
		SendValidationError(c, result)
		return
	}
	req.Page = page
	req.PageSize = pageSize

	var documents []model.Document
	totalCount := 0

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if instance, err := concreteEngine.GetIndex(indexName); err == nil {
			if engineInstance, ok := instance.(*engine.IndexInstance); ok {
				allDocs := engineInstance.DocumentStore.Docs
				totalCount = len(allDocs)

				// Calculate pagination
				startIndex := (req.Page - 1) * req.PageSize
				endIndex := startIndex + req.PageSize

				i := 0
				for _, doc := range allDocs {
					if i >= startIndex && i < endIndex {
						documents = append(documents, doc)
					}
					i++
					if i >= endIndex {
						break
					}
				}
			}
		}
	}

	response := gin.H{
		"documents": documents,
		"total":     totalCount,
		"page":      req.Page,
		"page_size": req.PageSize,
		"pages":     (totalCount + req.PageSize - 1) / req.PageSize,
	}

	c.JSON(http.StatusOK, response)
}

// GetDocumentHandler retrieves a specific document by ID
func (api *API) GetDocumentHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	documentId := c.Param("documentId")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	// Validate document ID
	if result := ValidateDocumentID(documentId); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	var document model.Document
	found := false

	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		if instance, err := concreteEngine.GetIndex(indexName); err == nil {
			if engineInstance, ok := instance.(*engine.IndexInstance); ok {
				for _, doc := range engineInstance.DocumentStore.Docs {
					if docID, exists := doc.GetDocumentID(); exists && docID == documentId {
						document = doc
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		SendDocumentNotFoundError(c, documentId, indexName)
		return
	}

	c.JSON(http.StatusOK, document)
}

// DeleteDocumentHandler deletes a specific document by ID
func (api *API) DeleteDocumentHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	documentId := c.Param("documentId")

	// Validate index name
	if result := ValidateIndexName(indexName); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	// Validate document ID
	if result := ValidateDocumentID(documentId); result.HasErrors() {
		SendValidationError(c, result)
		return
	}

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, internalErrors.ErrIndexNotFound) {
			SendIndexNotFoundError(c, indexName)
			return
		}
		SendInternalError(c, "get index", err)
		return
	}

	// Delete document asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteDocumentAsync(indexName, documentId)
		if err != nil {
			if errors.Is(err, internalErrors.ErrDocumentNotFound) {
				SendDocumentNotFoundError(c, documentId, indexName)
				return
			}
			SendJobExecutionError(c, "document deletion", err)
			return
		}

		// Return job ID with 202 Accepted status
		c.JSON(http.StatusAccepted, gin.H{
			"status":      "accepted",
			"message":     fmt.Sprintf("Document deletion started for document '%s' in index '%s'", documentId, indexName),
			"job_id":      jobID,
			"document_id": documentId,
		})
	} else {
		indexAccessor, _ := api.engine.GetIndex(indexName)
		err = indexAccessor.DeleteDocument(documentId)
		if err != nil {
			if errors.Is(err, internalErrors.ErrDocumentNotFound) {
				SendDocumentNotFoundError(c, documentId, indexName)
				return
			}
			SendIndexingError(c, "delete document", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Document '" + documentId + "' deleted from index '" + indexName + "'"})
	}
}

package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
)

// AddDocumentsHandler handles adding/updating documents in an index.
func (api *API) AddDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Read the raw JSON data first
	var rawData interface{}
	if err := c.ShouldBindJSON(&rawData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d is not a valid object", i)})
				return
			}
		}
	} else if docMap, isMap := rawData.(map[string]interface{}); isMap {
		// Handle single document
		docs = []model.Document{docMap}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. Expecting a document object or an array of documents"})
		return
	}

	if len(docs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No documents provided"})
		return
	}

	// Process documents: ensure documentID is present and is a valid string.
	for i := range docs {
		docMap := docs[i] // docs[i] is a map[string]interface{}

		// Handle documentID: must be present and valid (no auto-generation)
		uuidVal, uuidExists := docMap["documentID"]
		if !uuidExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d must have a 'documentID' field", i)})
			return
		}

		var finalDocumentID string
		switch u := uuidVal.(type) {
		case string:
			if strings.TrimSpace(u) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d has empty or whitespace-only documentID string", i)})
				return
			}
			finalDocumentID = strings.TrimSpace(u)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Document at index %d has documentID with unexpected type: %T (expected string)", i, uuidVal)})
			return
		}
		docMap["documentID"] = finalDocumentID // Ensure the map has a clean string for the indexing service.

		// documentID is the only required field; all others depend on index configuration
	}

	// Add documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.AddDocumentsAsync(indexName, docs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document addition: " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add documents to index '" + indexName + "': " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d document(s) added/updated in index '%s'", len(docs), indexName)})
	}
}

// DeleteAllDocumentsHandler handles the request to delete all documents from an index.
func (api *API) DeleteAllDocumentsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Delete all documents asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteAllDocumentsAsync(indexName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document deletion: " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all documents from index '" + indexName + "': " + err.Error()})
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
	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	var req DocumentListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}

	// Set defaults
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100 // Maximum page size
	}

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

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
		return
	}

	c.JSON(http.StatusOK, document)
}

// DeleteDocumentHandler deletes a specific document by ID
func (api *API) DeleteDocumentHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	documentId := c.Param("documentId")

	_, err := api.engine.GetIndex(indexName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index '" + indexName + "' not found"})
		return
	}

	// Delete document asynchronously
	var jobID string
	if concreteEngine, ok := api.engine.(*engine.Engine); ok {
		jobID, err = concreteEngine.DeleteDocumentAsync(indexName, documentId)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start async document deletion: " + err.Error()})
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
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Document '" + documentId + "' not found in index '" + indexName + "'"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document '" + documentId + "' from index '" + indexName + "': " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Document '" + documentId + "' deleted from index '" + indexName + "'"})
	}
}

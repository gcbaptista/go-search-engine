package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/search"
	"github.com/gcbaptista/go-search-engine/model"
)

// CreateIndexAsync creates a new index asynchronously.
func (e *Engine) CreateIndexAsync(settings config.IndexSettings) (string, error) {
	if settings.Name == "" {
		return "", fmt.Errorf("index name cannot be empty")
	}

	e.mu.RLock()
	if _, exists := e.indexes[settings.Name]; exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' already exists", settings.Name)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeCreateIndex, settings.Name, map[string]string{
		"operation": "create_index",
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeCreateIndexJob(ctx, settings, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start create index job: %w", err)
	}

	return jobID, nil
}

// executeCreateIndexJob executes the create index job.
func (e *Engine) executeCreateIndexJob(_ context.Context, settings config.IndexSettings, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check that index doesn't exist
	if _, exists := e.indexes[settings.Name]; exists {
		return fmt.Errorf("index named '%s' already exists", settings.Name)
	}

	// Create in-memory instance first
	instance, err := NewIndexInstance(settings)
	if err != nil {
		return fmt.Errorf("failed to create new index instance for '%s': %w", settings.Name, err)
	}

	// Initialize the searcher
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service for new index '%s': %w", settings.Name, err)
	}
	instance.SetSearcher(searchService)

	// Persist the initial state
	if err := e.persistUpdatedIndexUnsafe(settings.Name, settings, instance); err != nil {
		return fmt.Errorf("failed to persist new index '%s': %w", settings.Name, err)
	}

	e.indexes[settings.Name] = instance
	log.Printf("Index '%s' created and persisted asynchronously.", settings.Name)
	return nil
}

// DeleteIndexAsync deletes an index asynchronously.
func (e *Engine) DeleteIndexAsync(name string) (string, error) {
	e.mu.RLock()
	if _, exists := e.indexes[name]; !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", name)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeDeleteIndex, name, map[string]string{
		"operation": "delete_index",
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteIndexJob(ctx, name, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start delete index job: %w", err)
	}

	return jobID, nil
}

// executeDeleteIndexJob executes the delete index job.
func (e *Engine) executeDeleteIndexJob(_ context.Context, name string, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.indexes[name]; !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	// Remove from memory
	delete(e.indexes, name)

	// Remove from disk
	indexPath := filepath.Join(e.dataDir, name)
	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to remove index directory %s: %w", indexPath, err)
	}

	log.Printf("Index '%s' deleted successfully (async).", name)
	return nil
}

// AddDocumentsAsync adds documents to an index asynchronously.
func (e *Engine) AddDocumentsAsync(indexName string, docs []model.Document) (string, error) {
	e.mu.RLock()
	if _, exists := e.indexes[indexName]; !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeAddDocuments, indexName, map[string]string{
		"operation":      "add_documents",
		"document_count": fmt.Sprintf("%d", len(docs)),
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeAddDocumentsJob(ctx, indexName, docs, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start add documents job: %w", err)
	}

	return jobID, nil
}

// executeAddDocumentsJob executes the add documents job.
func (e *Engine) executeAddDocumentsJob(_ context.Context, indexName string, docs []model.Document, jobID string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found", indexName)
	}

	// Update progress
	e.jobManager.UpdateJobProgress(jobID, 0, len(docs), "Starting document addition")

	// Add documents
	if err := instance.AddDocuments(docs); err != nil {
		return fmt.Errorf("failed to add documents to index '%s': %w", indexName, err)
	}

	// Update progress
	e.jobManager.UpdateJobProgress(jobID, len(docs), len(docs), "Documents added successfully")

	// Persist the updated index
	e.mu.RLock()
	err := e.persistUpdatedIndexUnsafe(indexName, *instance.settings, instance)
	e.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to persist updated index '%s': %w", indexName, err)
	}

	log.Printf("Added %d documents to index '%s' (async).", len(docs), indexName)
	return nil
}

// RenameIndexAsync renames an index asynchronously.
func (e *Engine) RenameIndexAsync(oldName, newName string) (string, error) {
	if oldName == newName {
		return "", fmt.Errorf("old name and new name are the same: '%s'", oldName)
	}

	e.mu.RLock()
	if _, exists := e.indexes[oldName]; !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", oldName)
	}
	if _, exists := e.indexes[newName]; exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' already exists", newName)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeRenameIndex, oldName, map[string]string{
		"operation": "rename_index",
		"old_name":  oldName,
		"new_name":  newName,
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeRenameIndexJob(ctx, oldName, newName, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start rename index job: %w", err)
	}

	return jobID, nil
}

// executeRenameIndexJob executes the rename index job.
func (e *Engine) executeRenameIndexJob(_ context.Context, oldName, newName string, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[oldName]
	if !exists {
		return fmt.Errorf("index named '%s' not found", oldName)
	}

	if _, exists := e.indexes[newName]; exists {
		return fmt.Errorf("index named '%s' already exists", newName)
	}

	// Update the settings with the new name
	newSettings := *instance.settings
	newSettings.Name = newName

	// Create new directory and persist with new name
	if err := e.persistUpdatedIndexUnsafe(newName, newSettings, instance); err != nil {
		return fmt.Errorf("failed to persist renamed index: %w", err)
	}

	// Update in-memory settings
	instance.settings.Name = newName

	// Update the map
	e.indexes[newName] = instance
	delete(e.indexes, oldName)

	// Remove old directory
	oldIndexPath := filepath.Join(e.dataDir, oldName)
	if err := os.RemoveAll(oldIndexPath); err != nil {
		log.Printf("Warning: Failed to remove old index directory %s: %v", oldIndexPath, err)
		// Don't return error as the rename was successful
	}

	log.Printf("Index renamed from '%s' to '%s' successfully (async).", oldName, newName)
	return nil
}

// DeleteAllDocumentsAsync deletes all documents from an index asynchronously.
func (e *Engine) DeleteAllDocumentsAsync(indexName string) (string, error) {
	e.mu.RLock()
	if _, exists := e.indexes[indexName]; !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeDeleteAllDocs, indexName, map[string]string{
		"operation": "delete_all_documents",
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteAllDocumentsJob(ctx, indexName, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start delete all documents job: %w", err)
	}

	return jobID, nil
}

// executeDeleteAllDocumentsJob executes the delete all documents job.
func (e *Engine) executeDeleteAllDocumentsJob(_ context.Context, indexName string, _ string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found", indexName)
	}

	// Delete all documents
	if err := instance.DeleteAllDocuments(); err != nil {
		return fmt.Errorf("failed to delete all documents from index '%s': %w", indexName, err)
	}

	// Persist the updated index
	e.mu.RLock()
	err := e.persistUpdatedIndexUnsafe(indexName, *instance.settings, instance)
	e.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to persist updated index '%s': %w", indexName, err)
	}

	log.Printf("Deleted all documents from index '%s' (async).", indexName)
	return nil
}

// DeleteDocumentAsync deletes a specific document from an index asynchronously.
func (e *Engine) DeleteDocumentAsync(indexName, documentID string) (string, error) {
	e.mu.RLock()
	if _, exists := e.indexes[indexName]; !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}
	e.mu.RUnlock()

	jobID := e.jobManager.CreateJob(model.JobTypeDeleteDocument, indexName, map[string]string{
		"operation":   "delete_document",
		"document_id": documentID,
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteDocumentJob(ctx, indexName, documentID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start delete document job: %w", err)
	}

	return jobID, nil
}

// executeDeleteDocumentJob executes the delete document job.
func (e *Engine) executeDeleteDocumentJob(_ context.Context, indexName, documentID string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found", indexName)
	}

	// Delete the document
	if err := instance.DeleteDocument(documentID); err != nil {
		return fmt.Errorf("failed to delete document '%s' from index '%s': %w", documentID, indexName, err)
	}

	// Persist the updated index
	e.mu.RLock()
	err := e.persistUpdatedIndexUnsafe(indexName, *instance.settings, instance)
	e.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to persist updated index '%s': %w", indexName, err)
	}

	log.Printf("Deleted document '%s' from index '%s' (async).", documentID, indexName)
	return nil
}

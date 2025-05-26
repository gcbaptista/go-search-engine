package engine

import (
	"context"
	"fmt"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/search"
	"github.com/gcbaptista/go-search-engine/model"
)

// UpdateIndexSettings updates the settings for an index.
func (e *Engine) UpdateIndexSettings(name string, newSettings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	// Update settings
	*instance.settings = newSettings

	// Recreate search service with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service with new settings: %w", err)
	}
	instance.SetSearcher(searchService)

	// Persist updated settings
	return e.persistUpdatedIndexUnsafe(name, newSettings, instance)
}

// UpdateIndexSettingsWithReindex updates settings and performs a full reindex.
func (e *Engine) UpdateIndexSettingsWithReindex(name string, newSettings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	// Extract all documents before reindexing
	docs := e.extractAllDocumentsUnsafe(instance)

	// Clear the index
	if err := instance.DeleteAllDocuments(); err != nil {
		return fmt.Errorf("failed to clear index for reindexing: %w", err)
	}

	// Update settings
	*instance.settings = newSettings

	// Recreate search service with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service with new settings: %w", err)
	}
	instance.SetSearcher(searchService)

	// Re-add all documents
	if len(docs) > 0 {
		if err := instance.AddDocuments(docs); err != nil {
			return fmt.Errorf("failed to re-add documents during reindexing: %w", err)
		}
	}

	// Persist updated index
	return e.persistUpdatedIndexUnsafe(name, newSettings, instance)
}

// UpdateIndexSettingsWithAsyncReindex updates settings and performs async reindexing if needed.
func (e *Engine) UpdateIndexSettingsWithAsyncReindex(name string, newSettings config.IndexSettings) (string, error) {
	e.mu.RLock()
	instance, exists := e.indexes[name]
	if !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", name)
	}
	oldSettings := *instance.settings
	e.mu.RUnlock()

	// Check if full reindexing is required
	if e.requiresFullReindexing(oldSettings, newSettings) {
		// Submit async reindex job
		jobID := e.jobManager.CreateJob(model.JobTypeReindex, name, map[string]string{
			"operation": "settings_update_with_reindex",
		})

		err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
			return e.executeReindexJob(ctx, name, newSettings, jobID)
		})
		if err != nil {
			return "", fmt.Errorf("failed to start reindex job: %w", err)
		}

		return jobID, nil
	}

	// For search-time settings, submit a lighter job
	jobID := e.jobManager.CreateJob(model.JobTypeUpdateSettings, name, map[string]string{
		"operation": "search_time_settings_update",
	})

	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeSearchTimeSettingsUpdateJob(ctx, name, newSettings, jobID)
	})
	if err != nil {
		return "", fmt.Errorf("failed to start settings update job: %w", err)
	}

	return jobID, nil
}

// requiresFullReindexing determines if settings changes require full reindexing.
func (e *Engine) requiresFullReindexing(oldSettings, newSettings config.IndexSettings) bool {
	// Check if core indexing settings changed
	if !slicesEqual(oldSettings.SearchableFields, newSettings.SearchableFields) {
		return true
	}
	if !slicesEqual(oldSettings.FilterableFields, newSettings.FilterableFields) {
		return true
	}
	if !rankingCriteriaEqual(oldSettings.RankingCriteria, newSettings.RankingCriteria) {
		return true
	}
	if oldSettings.MinWordSizeFor1Typo != newSettings.MinWordSizeFor1Typo {
		return true
	}
	if oldSettings.MinWordSizeFor2Typos != newSettings.MinWordSizeFor2Typos {
		return true
	}
	return false
}

// executeSearchTimeSettingsUpdateJob executes a search-time settings update job.
func (e *Engine) executeSearchTimeSettingsUpdateJob(ctx context.Context, name string, newSettings config.IndexSettings, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	// Update settings
	*instance.settings = newSettings

	// Recreate search service with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service with new settings: %w", err)
	}
	instance.SetSearcher(searchService)

	// Persist updated settings
	return e.persistUpdatedIndexUnsafe(name, newSettings, instance)
}

// executeReindexJob executes a full reindex job.
func (e *Engine) executeReindexJob(ctx context.Context, name string, newSettings config.IndexSettings, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	// Extract all documents before reindexing
	docs := e.extractAllDocumentsUnsafe(instance)

	// Clear the index
	if err := instance.DeleteAllDocuments(); err != nil {
		return fmt.Errorf("failed to clear index for reindexing: %w", err)
	}

	// Update settings
	*instance.settings = newSettings

	// Recreate search service with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service with new settings: %w", err)
	}
	instance.SetSearcher(searchService)

	// Re-add all documents
	if len(docs) > 0 {
		if err := instance.AddDocuments(docs); err != nil {
			return fmt.Errorf("failed to re-add documents during reindexing: %w", err)
		}
	}

	// Persist updated index
	return e.persistUpdatedIndexUnsafe(name, newSettings, instance)
}

// Helper function to compare string slices
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Helper function to compare ranking criteria slices
func rankingCriteriaEqual(a, b []config.RankingCriterion) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Field != b[i].Field || a[i].Order != b[i].Order {
			return false
		}
	}
	return true
}

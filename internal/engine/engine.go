package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
	"github.com/gcbaptista/go-search-engine/internal/jobs"
	"github.com/gcbaptista/go-search-engine/internal/persistence"
	"github.com/gcbaptista/go-search-engine/internal/search"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gcbaptista/go-search-engine/store"
)

const (
	dataDirPerm       = 0755
	settingsFile      = "settings.gob"
	invertedIndexFile = "inverted_index.gob"
	documentStoreFile = "document_store.gob"
)

// Engine manages multiple search indexes.
// It implements the services.IndexManager interface.
type Engine struct {
	mu         sync.RWMutex
	indexes    map[string]*IndexInstance
	dataDir    string
	jobManager *jobs.Manager
}

// NewEngine creates a new search engine orchestrator.
func NewEngine(dataDir string) *Engine {
	eng := &Engine{
		indexes:    make(map[string]*IndexInstance),
		dataDir:    dataDir,
		jobManager: jobs.NewManager(2), // Allow max 2 concurrent reindexing jobs
	}
	eng.jobManager.Start()
	if err := os.MkdirAll(dataDir, dataDirPerm); err != nil {
		log.Printf("Warning: Could not create data directory %s: %v. Proceeding without persistence for new indexes if loading fails.", dataDir, err)
	}
	eng.loadIndexesFromDisk()
	return eng
}

func (e *Engine) loadIndexesFromDisk() {
	log.Printf("Loading indexes from disk: %s", e.dataDir)
	items, err := os.ReadDir(e.dataDir)
	if err != nil {
		log.Printf("Warning: Failed to read data directory %s: %v. No indexes loaded.", e.dataDir, err)
		return
	}

	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		indexName := item.Name()
		indexPath := filepath.Join(e.dataDir, indexName)
		log.Printf("Attempting to load index: %s", indexName)

		var settings config.IndexSettings
		settingsPath := filepath.Join(indexPath, settingsFile)
		if err := persistence.LoadGob(settingsPath, &settings); err != nil {
			log.Printf("Warning: Failed to load settings for index %s from %s: %v. Skipping this index.", indexName, settingsPath, err)
			continue
		}

		// Validate settings name matches directory name
		if settings.Name != indexName {
			log.Printf("Warning: Index name in settings ('%s') does not match directory name ('%s') for path %s. Skipping this index.", settings.Name, indexName, indexPath)
			continue
		}

		docStore := &store.DocumentStore{}
		dsPath := filepath.Join(indexPath, documentStoreFile)
		if err := persistence.LoadGob(dsPath, docStore); err != nil && err != os.ErrNotExist {
			log.Printf("Warning: Failed to load document store for index %s from %s: %v. Proceeding with empty store.", indexName, dsPath, err)
			// Initialize to empty if load failed but not due to file not existing (e.g. corrupted file)
			docStore.Docs = make(map[uint32]model.Document)
			docStore.ExternalIDtoInternalID = make(map[string]uint32)
		} else if err == os.ErrNotExist {
			log.Printf("Info: Document store file %s not found for index %s. Initializing empty store.", dsPath, indexName)
			docStore.Docs = make(map[uint32]model.Document)
			docStore.ExternalIDtoInternalID = make(map[string]uint32)
		}

		invIndex := &index.InvertedIndex{Settings: &settings} // Settings must be linked here
		iiPath := filepath.Join(indexPath, invertedIndexFile)
		if err := persistence.LoadGob(iiPath, invIndex); err != nil && err != os.ErrNotExist {
			log.Printf("Warning: Failed to load inverted index for index %s from %s: %v. Proceeding with empty index.", indexName, iiPath, err)
			invIndex.Index = make(map[string]index.PostingList) // Init to empty if corrupted
		} else if err == os.ErrNotExist {
			log.Printf("Info: Inverted index file %s not found for index %s. Initializing empty index.", iiPath, indexName)
			invIndex.Index = make(map[string]index.PostingList)
		}

		indexerService, err := indexing.NewService(invIndex, docStore)
		if err != nil {
			log.Printf("Error creating indexer service for loaded index %s: %v. Skipping.", indexName, err)
			continue
		}

		searchService, err := search.NewService(invIndex, docStore, &settings)
		if err != nil {
			log.Printf("Error creating search service for loaded index %s: %v. Skipping.", indexName, err)
			continue
		}

		instance := &IndexInstance{
			settings:      &settings,
			InvertedIndex: invIndex,
			DocumentStore: docStore,
			indexer:       indexerService,
			searcher:      searchService, // Assign loaded/initialized searcher
		}

		e.indexes[indexName] = instance
		log.Printf("Successfully loaded index: %s", indexName)
	}
}

// CreateIndex creates a new index with the given settings and persists it.
func (e *Engine) CreateIndex(settings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if settings.Name == "" {
		return fmt.Errorf("index name cannot be empty")
	}
	if _, exists := e.indexes[settings.Name]; exists {
		return fmt.Errorf("index named '%s' already exists", settings.Name)
	}

	// Create in-memory instance first
	instance, err := NewIndexInstance(settings) // This initializes sub-components
	if err != nil {
		return fmt.Errorf("failed to create new index instance for '%s': %w", settings.Name, err)
	}

	// Initialize the searcher (as NewIndexInstance doesn't do it anymore to avoid cyclic deps during basic init)
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service for new index '%s': %w", settings.Name, err)
	}
	instance.SetSearcher(searchService)

	// Persist the initial state
	indexPath := filepath.Join(e.dataDir, settings.Name)
	if err := os.MkdirAll(indexPath, dataDirPerm); err != nil {
		return fmt.Errorf("failed to create directory for index %s: %w", settings.Name, err)
	}

	if err := persistence.SaveGob(filepath.Join(indexPath, settingsFile), settings); err != nil {
		return fmt.Errorf("failed to save settings for index %s: %w", settings.Name, err)
	}
	// Save empty but initialized InvertedIndex (via instance.InvertedIndex which has custom Gob)
	if err := persistence.SaveGob(filepath.Join(indexPath, invertedIndexFile), instance.InvertedIndex); err != nil {
		return fmt.Errorf("failed to save initial inverted index for %s: %w", settings.Name, err)
	}
	// Save empty but initialized DocumentStore (via instance.DocumentStore which has custom Gob)
	if err := persistence.SaveGob(filepath.Join(indexPath, documentStoreFile), instance.DocumentStore); err != nil {
		return fmt.Errorf("failed to save initial document store for %s: %w", settings.Name, err)
	}

	e.indexes[settings.Name] = instance
	log.Printf("Index '%s' created and persisted.", settings.Name)
	return nil
}

// GetIndex retrieves an index by its name.
func (e *Engine) GetIndex(name string) (services.IndexAccessor, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.indexes[name]
	if !exists {
		return nil, fmt.Errorf("index named '%s' not found", name)
	}
	return instance, nil
}

// GetIndexSettings retrieves the settings for a specific index.
func (e *Engine) GetIndexSettings(name string) (config.IndexSettings, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.indexes[name]
	if !exists {
		return config.IndexSettings{}, fmt.Errorf("index named '%s' not found", name)
	}
	return *instance.settings, nil // Return a copy
}

// UpdateIndexSettings updates the settings for an existing index and persists them.
func (e *Engine) UpdateIndexSettings(name string, newSettings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found, cannot update settings", name)
	}

	// Validate that the name in newSettings matches the index name if provided and not empty
	if newSettings.Name != "" && newSettings.Name != name {
		return fmt.Errorf("cannot change index name from '%s' to '%s' during settings update", name, newSettings.Name)
	}
	newSettings.Name = name // Ensure the name remains consistent

	// Update in-memory settings
	instance.settings = &newSettings

	// Re-initialize services that depend on settings
	// Update the searcher with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, &newSettings)
	if err != nil {
		// Log critical error for inconsistent state
		log.Printf("CRITICAL: Failed to re-initialize search service for index '%s' after settings update. Index might be in an inconsistent state: %v", name, err)
		return fmt.Errorf("failed to update search service with new settings for '%s': %w", name, err)
	}
	instance.searcher = searchService // Update the searcher in the instance

	// Persist updated settings
	indexPath := filepath.Join(e.dataDir, name)
	settingsPath := filepath.Join(indexPath, settingsFile)
	if err := persistence.SaveGob(settingsPath, newSettings); err != nil {
		// Similar to above, inconsistent state if persistence fails after in-memory update.
		log.Printf("CRITICAL: Failed to persist updated settings for index '%s'. In-memory settings updated, but disk is stale: %v", name, err)
		return fmt.Errorf("failed to save updated settings for index '%s': %w", name, err)
	}

	log.Printf("Settings for index '%s' updated and persisted.", name)
	// Existing documents are not automatically re-indexed
	return nil
}

// UpdateIndexSettingsWithReindex updates index settings and automatically reindexes documents
// when core settings that affect indexing are changed.
func (e *Engine) UpdateIndexSettingsWithReindex(name string, newSettings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found, cannot update settings", name)
	}

	// Validate that the name in newSettings matches the index name
	if newSettings.Name != "" && newSettings.Name != name {
		return fmt.Errorf("cannot change index name from '%s' to '%s' during settings update", name, newSettings.Name)
	}
	newSettings.Name = name

	// Extract all existing documents before clearing the index
	existingDocs := e.extractAllDocumentsUnsafe(instance)
	log.Printf("Extracted %d documents for reindexing from index '%s'", len(existingDocs), name)

	// Clear the inverted index to start fresh
	instance.InvertedIndex.Index = make(map[string]index.PostingList)
	log.Printf("Cleared inverted index for reindexing of index '%s'", name)

	// Update settings
	instance.settings = &newSettings

	// Re-initialize services with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, &newSettings)
	if err != nil {
		log.Printf("CRITICAL: Failed to re-initialize search service for index '%s' during reindex: %v", name, err)
		return fmt.Errorf("failed to re-initialize search service with new settings for '%s': %w", name, err)
	}
	instance.searcher = searchService

	indexingService, err := indexing.NewService(instance.InvertedIndex, instance.DocumentStore)
	if err != nil {
		log.Printf("CRITICAL: Failed to re-initialize indexing service for index '%s' during reindex: %v", name, err)
		return fmt.Errorf("failed to re-initialize indexing service with new settings for '%s': %w", name, err)
	}
	instance.indexer = indexingService

	// Reindex all documents with new settings
	if len(existingDocs) > 0 {
		log.Printf("Starting reindexing of %d documents for index '%s'", len(existingDocs), name)
		if err := instance.indexer.AddDocuments(existingDocs); err != nil {
			log.Printf("CRITICAL: Failed to reindex documents for index '%s': %v", name, err)
			return fmt.Errorf("failed to reindex documents for '%s': %w", name, err)
		}
		log.Printf("Successfully reindexed %d documents for index '%s'", len(existingDocs), name)
	}

	// Persist updated settings and reindexed data
	if err := e.persistUpdatedIndexUnsafe(name, newSettings, instance); err != nil {
		log.Printf("CRITICAL: Failed to persist reindexed data for index '%s': %v", name, err)
		return fmt.Errorf("failed to persist reindexed data for '%s': %w", name, err)
	}

	log.Printf("Settings for index '%s' updated with reindexing completed and persisted.", name)
	return nil
}

// UpdateIndexSettingsWithAsyncReindex updates index settings and schedules async reindexing
// when core settings that affect indexing are changed. Returns immediately with a job ID.
func (e *Engine) UpdateIndexSettingsWithAsyncReindex(name string, newSettings config.IndexSettings) (string, error) {
	e.mu.RLock()
	instance, exists := e.indexes[name]
	if !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found, cannot update settings", name)
	}
	oldSettings := *instance.settings
	e.mu.RUnlock()

	// Validate that the name in newSettings matches the index name
	if newSettings.Name != "" && newSettings.Name != name {
		return "", fmt.Errorf("cannot change index name from '%s' to '%s' during settings update", name, newSettings.Name)
	}
	newSettings.Name = name

	// Determine if full reindexing is actually needed
	requiresFullReindex := e.requiresFullReindexing(oldSettings, newSettings)

	if requiresFullReindex {
		// Core indexing structure changed - full reindexing needed
		jobID := e.jobManager.CreateJob(model.JobTypeReindex, name, map[string]string{
			"operation": "update_settings_with_reindex",
			"reason":    "Core settings changed requiring full reindexing",
		})

		err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
			return e.executeReindexJob(ctx, name, newSettings, job.ID)
		})

		if err != nil {
			return "", fmt.Errorf("failed to start async reindexing job: %w", err)
		}

		log.Printf("Started async full reindexing job %s for index '%s' (core settings changed)", jobID, name)
		return jobID, nil
	} else {
		// Only typo tolerance or field-level settings changed - search-time update
		jobID := e.jobManager.CreateJob(model.JobTypeUpdateSettings, name, map[string]string{
			"operation": "update_settings_search_time",
			"reason":    "Search-time settings changed - no reindexing needed",
		})

		err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
			return e.executeSearchTimeSettingsUpdateJob(ctx, name, newSettings, job.ID)
		})

		if err != nil {
			return "", fmt.Errorf("failed to start search-time settings update job: %w", err)
		}

		log.Printf("Started search-time settings update job %s for index '%s' (search behavior changes only)", jobID, name)
		return jobID, nil
	}
}

// requiresFullReindexing determines if settings changes require full reindexing
func (e *Engine) requiresFullReindexing(oldSettings, newSettings config.IndexSettings) bool {
	// Check if core indexing structure changed
	if !slicesEqual(oldSettings.SearchableFields, newSettings.SearchableFields) {
		return true // Searchable fields affect which fields get indexed
	}

	if !slicesEqual(oldSettings.FilterableFields, newSettings.FilterableFields) {
		return true // Filterable fields affect indexing structure
	}

	if !rankingCriteriaEqual(oldSettings.RankingCriteria, newSettings.RankingCriteria) {
		return true // Ranking criteria affect search result ordering
	}

	// Typo tolerance settings DON'T require reindexing - they only affect search-time behavior
	// - MinWordSizeFor1Typo: affects eligibility during search, not indexing
	// - MinWordSizeFor2Typos: affects eligibility during search, not indexing

	// Field-level settings DON'T require reindexing - they affect search behavior only
	// - FieldsWithoutPrefixSearch: affects tokenization strategy but can be handled at search time
	// - NoTypoToleranceFields: affects search behavior only
	// - DistinctField: affects result deduplication only

	return false // No core indexing changes detected
}

// executeSearchTimeSettingsUpdateJob performs search-time settings update without reindexing
func (e *Engine) executeSearchTimeSettingsUpdateJob(ctx context.Context, name string, newSettings config.IndexSettings, jobID string) error {
	e.jobManager.UpdateJobProgress(jobID, 0, 4, "Starting search-time settings update")

	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found", name)
	}

	e.jobManager.UpdateJobProgress(jobID, 1, 4, "Updating settings in memory")

	// Update the settings in memory
	instance.settings = &newSettings

	e.jobManager.UpdateJobProgress(jobID, 2, 4, "Rebuilding search service with new settings")

	// Rebuild search service with new settings (this updates the typo finder)
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, &newSettings)
	if err != nil {
		return fmt.Errorf("failed to rebuild search service with new settings for '%s': %w", name, err)
	}
	instance.searcher = searchService

	e.jobManager.UpdateJobProgress(jobID, 3, 4, "Persisting settings to disk")

	// Persist the updated settings
	if err := e.persistUpdatedIndexUnsafe(name, newSettings, instance); err != nil {
		return fmt.Errorf("failed to persist updated settings for index '%s': %w", name, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 4, 4, "Settings update completed (no reindexing needed)")
	log.Printf("Search-time settings update completed for index '%s' (job %s) - search behavior updated without reindexing", name, jobID)
	return nil
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

// executeReindexJob performs the actual reindexing work in a background job
func (e *Engine) executeReindexJob(ctx context.Context, name string, newSettings config.IndexSettings, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, exists := e.indexes[name]
	if !exists {
		return fmt.Errorf("index named '%s' not found during reindexing", name)
	}

	// Extract all existing documents before clearing the index
	existingDocs := e.extractAllDocumentsUnsafe(instance)
	log.Printf("Extracted %d documents for reindexing from index '%s'", len(existingDocs), name)

	// Update progress
	e.jobManager.UpdateJobProgress(jobID, 0, len(existingDocs)+3, "Extracted documents, clearing index")

	// Clear the inverted index to start fresh
	instance.InvertedIndex.Index = make(map[string]index.PostingList)
	log.Printf("Cleared inverted index for reindexing of index '%s'", name)

	// Update settings
	instance.settings = &newSettings

	e.jobManager.UpdateJobProgress(jobID, 1, len(existingDocs)+3, "Reinitializing services")

	// Re-initialize services with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, &newSettings)
	if err != nil {
		log.Printf("CRITICAL: Failed to re-initialize search service for index '%s' during reindex: %v", name, err)
		return fmt.Errorf("failed to re-initialize search service with new settings for '%s': %w", name, err)
	}
	instance.searcher = searchService

	indexingService, err := indexing.NewService(instance.InvertedIndex, instance.DocumentStore)
	if err != nil {
		log.Printf("CRITICAL: Failed to re-initialize indexing service for index '%s' during reindex: %v", name, err)
		return fmt.Errorf("failed to re-initialize indexing service with new settings for '%s': %w", name, err)
	}
	instance.indexer = indexingService

	e.jobManager.UpdateJobProgress(jobID, 2, len(existingDocs)+3, "Reindexing documents")

	// Reindex all documents with new settings
	if len(existingDocs) > 0 {
		log.Printf("Starting reindexing of %d documents for index '%s'", len(existingDocs), name)

		// Process documents in batches to update progress and check for cancellation
		batchSize := 100
		for i := 0; i < len(existingDocs); i += batchSize {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return fmt.Errorf("reindexing cancelled")
			default:
			}

			end := i + batchSize
			if end > len(existingDocs) {
				end = len(existingDocs)
			}

			batch := existingDocs[i:end]
			if err := instance.indexer.AddDocuments(batch); err != nil {
				log.Printf("CRITICAL: Failed to reindex document batch %d-%d for index '%s': %v", i, end-1, name, err)
				return fmt.Errorf("failed to reindex documents for '%s': %w", name, err)
			}

			// Update progress
			e.jobManager.UpdateJobProgress(jobID, 2+end, len(existingDocs)+3, fmt.Sprintf("Reindexed %d/%d documents", end, len(existingDocs)))
		}

		log.Printf("Successfully reindexed %d documents for index '%s'", len(existingDocs), name)
	}

	e.jobManager.UpdateJobProgress(jobID, len(existingDocs)+2, len(existingDocs)+3, "Persisting changes")

	// Persist updated settings and reindexed data
	if err := e.persistUpdatedIndexUnsafe(name, newSettings, instance); err != nil {
		log.Printf("CRITICAL: Failed to persist reindexed data for index '%s': %v", name, err)
		return fmt.Errorf("failed to persist reindexed data for '%s': %w", name, err)
	}

	e.jobManager.UpdateJobProgress(jobID, len(existingDocs)+3, len(existingDocs)+3, "Reindexing completed")
	log.Printf("Settings for index '%s' updated with reindexing completed and persisted.", name)
	return nil
}

// GetJob returns job information by ID (implements JobManager interface)
func (e *Engine) GetJob(jobID string) (*model.Job, error) {
	return e.jobManager.GetJob(jobID)
}

// ListJobs returns jobs for an index, optionally filtered by status (implements JobManager interface)
func (e *Engine) ListJobs(indexName string, status *model.JobStatus) []*model.Job {
	return e.jobManager.ListJobs(indexName, status)
}

// extractAllDocumentsUnsafe extracts all documents from an index instance.
// Caller must hold the engine lock.
func (e *Engine) extractAllDocumentsUnsafe(instance *IndexInstance) []model.Document {
	docs := make([]model.Document, 0, len(instance.DocumentStore.Docs))
	for _, doc := range instance.DocumentStore.Docs {
		docs = append(docs, doc)
	}
	return docs
}

// persistUpdatedIndexUnsafe persists the updated index settings and data.
// Caller must hold the engine lock.
func (e *Engine) persistUpdatedIndexUnsafe(name string, settings config.IndexSettings, instance *IndexInstance) error {
	indexPath := filepath.Join(e.dataDir, name)

	// Save updated settings
	settingsPath := filepath.Join(indexPath, settingsFile)
	if err := persistence.SaveGob(settingsPath, settings); err != nil {
		return fmt.Errorf("failed to save updated settings: %w", err)
	}

	// Save reindexed inverted index
	if err := persistence.SaveGob(filepath.Join(indexPath, invertedIndexFile), instance.InvertedIndex); err != nil {
		return fmt.Errorf("failed to save reindexed inverted index: %w", err)
	}

	// Save document store (should be unchanged but save for consistency)
	if err := persistence.SaveGob(filepath.Join(indexPath, documentStoreFile), instance.DocumentStore); err != nil {
		return fmt.Errorf("failed to save document store: %w", err)
	}

	return nil
}

// DeleteIndex removes an index by its name from memory and disk.
func (e *Engine) DeleteIndex(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.indexes[name]; !exists {
		// To be idempotent, if not in memory, check if it exists on disk to remove
		indexPath := filepath.Join(e.dataDir, name)
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			return fmt.Errorf("index named '%s' not found in memory or on disk", name)
		}
	}
	// Safe to call delete even if key doesn't exist
	delete(e.indexes, name)

	indexPath := filepath.Join(e.dataDir, name)
	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to delete index data directory %s: %w", indexPath, err)
	}
	log.Printf("Index '%s' deleted from memory and disk.", name)
	return nil
}

// RenameIndex renames an index by moving its data directory and updating all internal references.
func (e *Engine) RenameIndex(oldName, newName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate input
	if oldName == "" || newName == "" {
		return fmt.Errorf("both old and new index names must be non-empty")
	}
	if oldName == newName {
		return fmt.Errorf("old and new index names are the same")
	}

	// Check if source index exists
	instance, exists := e.indexes[oldName]
	if !exists {
		return fmt.Errorf("index named '%s' not found", oldName)
	}

	// Check if target index name already exists
	if _, exists := e.indexes[newName]; exists {
		return fmt.Errorf("index named '%s' already exists", newName)
	}

	// Check if target directory already exists on disk
	newIndexPath := filepath.Join(e.dataDir, newName)
	if _, err := os.Stat(newIndexPath); err == nil {
		return fmt.Errorf("directory for index '%s' already exists on disk", newName)
	}

	oldIndexPath := filepath.Join(e.dataDir, oldName)

	// Move the directory
	if err := os.Rename(oldIndexPath, newIndexPath); err != nil {
		return fmt.Errorf("failed to rename index directory from '%s' to '%s': %w", oldIndexPath, newIndexPath, err)
	}

	// Update the settings with the new name
	newSettings := *instance.settings // Create a copy
	newSettings.Name = newName
	instance.settings = &newSettings

	// Update in-memory index reference
	e.indexes[newName] = instance
	delete(e.indexes, oldName)

	// Persist the updated settings to the new location
	settingsPath := filepath.Join(newIndexPath, settingsFile)
	if err := persistence.SaveGob(settingsPath, newSettings); err != nil {
		// Try to rollback the directory rename if settings save fails
		if rollbackErr := os.Rename(newIndexPath, oldIndexPath); rollbackErr != nil {
			log.Printf("CRITICAL: Failed to rollback directory rename after settings save failure. Index '%s' directory exists but not in memory. Manual intervention required: %v", newName, rollbackErr)
		} else {
			// Rollback in-memory changes
			e.indexes[oldName] = instance
			delete(e.indexes, newName)
			instance.settings.Name = oldName
		}
		return fmt.Errorf("failed to save updated settings after rename: %w", err)
	}

	log.Printf("Index successfully renamed from '%s' to '%s'", oldName, newName)
	return nil
}

// ListIndexes returns a list of names of all existing indexes.
func (e *Engine) ListIndexes() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.indexes))
	for name := range e.indexes {
		names = append(names, name)
	}
	// Only show loaded indexes for consistency
	return names
}

// PersistIndexData requests an index instance to save its current state.
// This should be called after modifications (e.g., AddDocuments).
func (e *Engine) PersistIndexData(indexName string) error {
	e.mu.RLock() // Lock to safely get the instance
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("cannot persist: index '%s' not found", indexName)
	}

	indexPath := filepath.Join(e.dataDir, indexName)

	// InvertedIndex and DocumentStore already have RLock in their GobEncode methods
	if err := persistence.SaveGob(filepath.Join(indexPath, invertedIndexFile), instance.InvertedIndex); err != nil {
		return fmt.Errorf("failed to save inverted index for %s: %w", indexName, err)
	}
	if err := persistence.SaveGob(filepath.Join(indexPath, documentStoreFile), instance.DocumentStore); err != nil {
		return fmt.Errorf("failed to save document store for %s: %w", indexName, err)
	}
	log.Printf("Data for index '%s' persisted.", indexName)
	return nil
}

// GetJobMetrics returns current job performance metrics (returns a copy without mutex)
func (e *Engine) GetJobMetrics() jobs.JobMetricsData {
	return e.jobManager.GetMetrics()
}

// GetJobSuccessRate returns the overall job success rate
func (e *Engine) GetJobSuccessRate() float64 {
	return e.jobManager.GetJobSuccessRate()
}

// GetCurrentWorkload returns the number of currently active jobs
func (e *Engine) GetCurrentWorkload() int64 {
	return e.jobManager.GetCurrentWorkload()
}

// CreateIndexAsync creates a new index asynchronously
func (e *Engine) CreateIndexAsync(settings config.IndexSettings) (string, error) {
	// Validate settings first
	if settings.Name == "" {
		return "", fmt.Errorf("index name cannot be empty")
	}

	e.mu.RLock()
	if _, exists := e.indexes[settings.Name]; exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' already exists", settings.Name)
	}
	e.mu.RUnlock()

	// Create job for async index creation
	jobID := e.jobManager.CreateJob(model.JobTypeCreateIndex, settings.Name, map[string]string{
		"operation":  "create_index",
		"index_name": settings.Name,
	})

	// Execute async index creation
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeCreateIndexJob(ctx, settings, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async index creation job: %w", err)
	}

	log.Printf("Started async index creation job %s for index '%s'", jobID, settings.Name)
	return jobID, nil
}

// executeCreateIndexJob performs the actual index creation work
func (e *Engine) executeCreateIndexJob(ctx context.Context, settings config.IndexSettings, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check index doesn't exist (race condition safety)
	if _, exists := e.indexes[settings.Name]; exists {
		return fmt.Errorf("index named '%s' already exists", settings.Name)
	}

	e.jobManager.UpdateJobProgress(jobID, 0, 3, "Creating index instance")

	// Create in-memory instance
	instance, err := NewIndexInstance(settings)
	if err != nil {
		return fmt.Errorf("failed to create new index instance for '%s': %w", settings.Name, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 1, 3, "Initializing search services")

	// Initialize the searcher
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, instance.settings)
	if err != nil {
		return fmt.Errorf("failed to create search service for new index '%s': %w", settings.Name, err)
	}
	instance.SetSearcher(searchService)

	e.jobManager.UpdateJobProgress(jobID, 2, 3, "Persisting index data")

	// Persist the initial state
	indexPath := filepath.Join(e.dataDir, settings.Name)
	if err := os.MkdirAll(indexPath, dataDirPerm); err != nil {
		return fmt.Errorf("failed to create directory for index %s: %w", settings.Name, err)
	}

	if err := persistence.SaveGob(filepath.Join(indexPath, settingsFile), settings); err != nil {
		return fmt.Errorf("failed to save settings for index %s: %w", settings.Name, err)
	}
	if err := persistence.SaveGob(filepath.Join(indexPath, invertedIndexFile), instance.InvertedIndex); err != nil {
		return fmt.Errorf("failed to save initial inverted index for %s: %w", settings.Name, err)
	}
	if err := persistence.SaveGob(filepath.Join(indexPath, documentStoreFile), instance.DocumentStore); err != nil {
		return fmt.Errorf("failed to save initial document store for %s: %w", settings.Name, err)
	}

	e.indexes[settings.Name] = instance
	e.jobManager.UpdateJobProgress(jobID, 3, 3, "Index creation completed")
	log.Printf("Index '%s' created and persisted via async job", settings.Name)
	return nil
}

// DeleteIndexAsync deletes an index asynchronously
func (e *Engine) DeleteIndexAsync(name string) (string, error) {
	e.mu.RLock()
	if _, exists := e.indexes[name]; !exists {
		e.mu.RUnlock()
		// Check if it exists on disk
		indexPath := filepath.Join(e.dataDir, name)
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			return "", fmt.Errorf("index named '%s' not found in memory or on disk", name)
		}
	}
	e.mu.RUnlock()

	// Create job for async index deletion
	jobID := e.jobManager.CreateJob(model.JobTypeDeleteIndex, name, map[string]string{
		"operation":  "delete_index",
		"index_name": name,
	})

	// Execute async index deletion
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteIndexJob(ctx, name, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async index deletion job: %w", err)
	}

	log.Printf("Started async index deletion job %s for index '%s'", jobID, name)
	return jobID, nil
}

// executeDeleteIndexJob performs the actual index deletion work
func (e *Engine) executeDeleteIndexJob(ctx context.Context, name string, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.jobManager.UpdateJobProgress(jobID, 0, 2, "Removing index from memory")

	delete(e.indexes, name)

	e.jobManager.UpdateJobProgress(jobID, 1, 2, "Deleting index files from disk")

	indexPath := filepath.Join(e.dataDir, name)
	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to delete index data directory %s: %w", indexPath, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 2, 2, "Index deletion completed")
	log.Printf("Index '%s' deleted from memory and disk via async job", name)
	return nil
}

// AddDocumentsAsync adds documents to an index asynchronously
func (e *Engine) AddDocumentsAsync(indexName string, docs []model.Document) (string, error) {
	e.mu.RLock()
	_, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}

	// Create job for async document addition
	jobID := e.jobManager.CreateJob(model.JobTypeAddDocuments, indexName, map[string]string{
		"operation":      "add_documents",
		"document_count": fmt.Sprintf("%d", len(docs)),
	})

	// Execute async document addition
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeAddDocumentsJob(ctx, indexName, docs, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async add documents job: %w", err)
	}

	log.Printf("Started async add documents job %s for index '%s' (%d documents)", jobID, indexName, len(docs))
	return jobID, nil
}

// executeAddDocumentsJob performs the actual document addition work
func (e *Engine) executeAddDocumentsJob(ctx context.Context, indexName string, docs []model.Document, jobID string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found during document addition", indexName)
	}

	e.jobManager.UpdateJobProgress(jobID, 0, len(docs)+1, "Starting document addition")

	// Process documents in batches for progress updates
	batchSize := 100
	for i := 0; i < len(docs); i += batchSize {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("document addition cancelled")
		default:
		}

		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := docs[i:end]
		if err := instance.AddDocuments(batch); err != nil {
			return fmt.Errorf("failed to add document batch %d-%d: %w", i, end-1, err)
		}

		e.jobManager.UpdateJobProgress(jobID, end, len(docs)+1, fmt.Sprintf("Added %d/%d documents", end, len(docs)))
	}

	e.jobManager.UpdateJobProgress(jobID, len(docs), len(docs)+1, "Persisting changes")

	// Persist changes
	if err := e.PersistIndexData(indexName); err != nil {
		log.Printf("Warning: Failed to persist data for index '%s' after adding documents: %v", indexName, err)
	}

	e.jobManager.UpdateJobProgress(jobID, len(docs)+1, len(docs)+1, "Document addition completed")
	log.Printf("Successfully added %d documents to index '%s' via async job", len(docs), indexName)
	return nil
}

// RenameIndexAsync renames an index asynchronously
func (e *Engine) RenameIndexAsync(oldName, newName string) (string, error) {
	// Validate input
	if oldName == "" || newName == "" {
		return "", fmt.Errorf("both old and new index names must be non-empty")
	}
	if oldName == newName {
		return "", fmt.Errorf("old and new index names are the same")
	}

	e.mu.RLock()
	_, exists := e.indexes[oldName]
	if !exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' not found", oldName)
	}
	if _, exists := e.indexes[newName]; exists {
		e.mu.RUnlock()
		return "", fmt.Errorf("index named '%s' already exists", newName)
	}
	e.mu.RUnlock()

	// Create job for async index rename
	jobID := e.jobManager.CreateJob(model.JobTypeRenameIndex, oldName, map[string]string{
		"operation": "rename_index",
		"old_name":  oldName,
		"new_name":  newName,
	})

	// Execute async index rename
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeRenameIndexJob(ctx, oldName, newName, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async index rename job: %w", err)
	}

	log.Printf("Started async index rename job %s: '%s' -> '%s'", jobID, oldName, newName)
	return jobID, nil
}

// executeRenameIndexJob performs the actual index rename work
func (e *Engine) executeRenameIndexJob(ctx context.Context, oldName, newName string, jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.jobManager.UpdateJobProgress(jobID, 0, 4, "Validating rename operation")

	instance, exists := e.indexes[oldName]
	if !exists {
		return fmt.Errorf("index named '%s' not found", oldName)
	}
	if _, exists := e.indexes[newName]; exists {
		return fmt.Errorf("index named '%s' already exists", newName)
	}

	e.jobManager.UpdateJobProgress(jobID, 1, 4, "Moving index directory")

	oldIndexPath := filepath.Join(e.dataDir, oldName)
	newIndexPath := filepath.Join(e.dataDir, newName)

	// Check if target directory already exists on disk
	if _, err := os.Stat(newIndexPath); err == nil {
		return fmt.Errorf("directory for index '%s' already exists on disk", newName)
	}

	// Move the directory
	if err := os.Rename(oldIndexPath, newIndexPath); err != nil {
		return fmt.Errorf("failed to rename index directory from '%s' to '%s': %w", oldIndexPath, newIndexPath, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 2, 4, "Updating index settings")

	// Update the settings with the new name
	newSettings := *instance.settings
	newSettings.Name = newName
	instance.settings = &newSettings

	e.jobManager.UpdateJobProgress(jobID, 3, 4, "Updating in-memory references")

	// Update in-memory index reference
	e.indexes[newName] = instance
	delete(e.indexes, oldName)

	// Persist the updated settings to the new location
	settingsPath := filepath.Join(newIndexPath, settingsFile)
	if err := persistence.SaveGob(settingsPath, newSettings); err != nil {
		// Try to rollback the directory rename if settings save fails
		if rollbackErr := os.Rename(newIndexPath, oldIndexPath); rollbackErr != nil {
			log.Printf("CRITICAL: Failed to rollback directory rename after settings save failure. Index '%s' directory exists but not in memory. Manual intervention required: %v", newName, rollbackErr)
		} else {
			// Rollback in-memory changes
			e.indexes[oldName] = instance
			delete(e.indexes, newName)
			instance.settings.Name = oldName
		}
		return fmt.Errorf("failed to save updated settings after rename: %w", err)
	}

	e.jobManager.UpdateJobProgress(jobID, 4, 4, "Index rename completed")
	log.Printf("Index successfully renamed from '%s' to '%s' via async job", oldName, newName)
	return nil
}

// DeleteAllDocumentsAsync deletes all documents from an index asynchronously
func (e *Engine) DeleteAllDocumentsAsync(indexName string) (string, error) {
	e.mu.RLock()
	_, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}

	// Create job for async document deletion
	jobID := e.jobManager.CreateJob(model.JobTypeDeleteAllDocs, indexName, map[string]string{
		"operation": "delete_all_documents",
	})

	// Execute async document deletion
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteAllDocumentsJob(ctx, indexName, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async delete all documents job: %w", err)
	}

	log.Printf("Started async delete all documents job %s for index '%s'", jobID, indexName)
	return jobID, nil
}

// executeDeleteAllDocumentsJob performs the actual document deletion work
func (e *Engine) executeDeleteAllDocumentsJob(ctx context.Context, indexName string, jobID string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found during document deletion", indexName)
	}

	e.jobManager.UpdateJobProgress(jobID, 0, 3, "Starting document deletion")

	// Get current document count for progress tracking
	docCount := len(instance.DocumentStore.Docs)

	e.jobManager.UpdateJobProgress(jobID, 1, 3, fmt.Sprintf("Deleting %d documents", docCount))

	// Clear all documents
	if err := instance.DeleteAllDocuments(); err != nil {
		return fmt.Errorf("failed to delete all documents from index '%s': %w", indexName, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 2, 3, "Persisting changes")

	// Persist changes
	if err := e.PersistIndexData(indexName); err != nil {
		log.Printf("Warning: Failed to persist data for index '%s' after deleting all documents: %v", indexName, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 3, 3, "Document deletion completed")
	log.Printf("Successfully deleted all documents from index '%s' via async job", indexName)
	return nil
}

// DeleteDocumentAsync deletes a specific document from an index asynchronously
func (e *Engine) DeleteDocumentAsync(indexName, documentID string) (string, error) {
	e.mu.RLock()
	_, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("index named '%s' not found", indexName)
	}

	// Create job for async document deletion
	jobID := e.jobManager.CreateJob(model.JobTypeDeleteDocument, indexName, map[string]string{
		"operation":   "delete_document",
		"document_id": documentID,
	})

	// Execute async document deletion
	err := e.jobManager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		return e.executeDeleteDocumentJob(ctx, indexName, documentID, job.ID)
	})

	if err != nil {
		return "", fmt.Errorf("failed to start async delete document job: %w", err)
	}

	log.Printf("Started async delete document job %s for index '%s' (document: %s)", jobID, indexName, documentID)
	return jobID, nil
}

// executeDeleteDocumentJob performs the actual single document deletion work
func (e *Engine) executeDeleteDocumentJob(ctx context.Context, indexName, documentID, jobID string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found during document deletion", indexName)
	}

	e.jobManager.UpdateJobProgress(jobID, 0, 3, "Starting document deletion")

	// Check if document exists
	instance.DocumentStore.Mu.RLock()
	_, exists = instance.DocumentStore.ExternalIDtoInternalID[documentID]
	instance.DocumentStore.Mu.RUnlock()

	if !exists {
		return fmt.Errorf("document '%s' not found in index '%s'", documentID, indexName)
	}

	e.jobManager.UpdateJobProgress(jobID, 1, 3, "Deleting document")

	// Delete the document
	if err := instance.DeleteDocument(documentID); err != nil {
		return fmt.Errorf("failed to delete document '%s' from index '%s': %w", documentID, indexName, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 2, 3, "Persisting changes")

	// Persist changes
	if err := e.PersistIndexData(indexName); err != nil {
		log.Printf("Warning: Failed to persist data for index '%s' after deleting document '%s': %v", indexName, documentID, err)
	}

	e.jobManager.UpdateJobProgress(jobID, 3, 3, "Document deletion completed")
	log.Printf("Successfully deleted document '%s' from index '%s' via async job", documentID, indexName)
	return nil
}

package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
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
	mu      sync.RWMutex
	indexes map[string]*IndexInstance
	dataDir string
}

// NewEngine creates a new search engine orchestrator.
func NewEngine(dataDir string) *Engine {
	eng := &Engine{
		indexes: make(map[string]*IndexInstance),
		dataDir: dataDir,
	}
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

		// Basic validation, settings name should match directory name
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

	// Re-initialize services that depend on settings if necessary.
	// For now, let's assume services can pick up changes from the shared settings pointer,
	// or they are robust enough. A more complex system might require re-initialization.
	// Update the searcher with new settings
	searchService, err := search.NewService(instance.InvertedIndex, instance.DocumentStore, &newSettings)
	if err != nil {
		// Attempt to revert settings change on error to maintain consistency
		// This is a simplified revert; a robust system might reload old settings
		// For now, logging the error is critical.
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
	// Note: Existing documents are NOT automatically re-indexed here. This must be done by the user.
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
	} else {
		delete(e.indexes, name)
	}

	indexPath := filepath.Join(e.dataDir, name)
	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to delete index data directory %s: %w", indexPath, err)
	}
	log.Printf("Index '%s' deleted from memory and disk.", name)
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
	// Optionally, could also list from disk if an index failed to load into memory
	// but for consistency, only show loaded indexes.
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

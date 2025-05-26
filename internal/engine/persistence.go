package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
	"github.com/gcbaptista/go-search-engine/internal/persistence"
	"github.com/gcbaptista/go-search-engine/internal/search"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/store"
)

const (
	dataDirPerm       = 0755
	settingsFile      = "settings.gob"
	invertedIndexFile = "inverted_index.gob"
	documentStoreFile = "document_store.gob"
)

// loadIndexesFromDisk loads all indexes from the data directory.
func (e *Engine) loadIndexesFromDisk() {
	log.Printf("Loading indexes from disk: %s", e.dataDir)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(e.dataDir, dataDirPerm); err != nil {
		log.Printf("Warning: Could not create data directory %s: %v. Proceeding without persistence for new indexes if loading fails.", e.dataDir, err)
	}

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

// PersistIndexData persists the data for a specific index to disk.
func (e *Engine) PersistIndexData(indexName string) error {
	e.mu.RLock()
	instance, exists := e.indexes[indexName]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("index named '%s' not found", indexName)
	}

	return e.persistUpdatedIndexUnsafe(indexName, *instance.settings, instance)
}

// persistUpdatedIndexUnsafe persists an index instance to disk.
// This method assumes the caller has appropriate locking.
func (e *Engine) persistUpdatedIndexUnsafe(name string, settings config.IndexSettings, instance *IndexInstance) error {
	indexPath := filepath.Join(e.dataDir, name)
	if err := os.MkdirAll(indexPath, dataDirPerm); err != nil {
		return fmt.Errorf("failed to create directory for index %s: %w", name, err)
	}

	if err := persistence.SaveGob(filepath.Join(indexPath, settingsFile), settings); err != nil {
		return fmt.Errorf("failed to save settings for index %s: %w", name, err)
	}
	if err := persistence.SaveGob(filepath.Join(indexPath, invertedIndexFile), instance.InvertedIndex); err != nil {
		return fmt.Errorf("failed to save inverted index for %s: %w", name, err)
	}
	if err := persistence.SaveGob(filepath.Join(indexPath, documentStoreFile), instance.DocumentStore); err != nil {
		return fmt.Errorf("failed to save document store for %s: %w", name, err)
	}

	return nil
}

// extractAllDocumentsUnsafe extracts all documents from an index instance.
// This method assumes the caller has appropriate locking.
func (e *Engine) extractAllDocumentsUnsafe(instance *IndexInstance) []model.Document {
	docs := make([]model.Document, 0, len(instance.DocumentStore.Docs))
	for _, doc := range instance.DocumentStore.Docs {
		docs = append(docs, doc)
	}
	return docs
}

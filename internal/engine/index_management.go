package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/internal/search"
)

// CreateIndex creates a new index with the given settings and persists it.
func (e *Engine) CreateIndex(settings config.IndexSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if settings.Name == "" {
		return fmt.Errorf("index name cannot be empty")
	}
	if _, exists := e.indexes[settings.Name]; exists {
		return errors.NewIndexAlreadyExistsError(settings.Name)
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
	if err := e.persistUpdatedIndexUnsafe(settings.Name, settings, instance); err != nil {
		return fmt.Errorf("failed to persist new index '%s': %w", settings.Name, err)
	}

	e.indexes[settings.Name] = instance
	log.Printf("Index '%s' created and persisted.", settings.Name)
	return nil
}

// DeleteIndex deletes an index and its data from disk.
func (e *Engine) DeleteIndex(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.indexes[name]; !exists {
		return errors.NewIndexNotFoundError(name)
	}

	// Remove from memory
	delete(e.indexes, name)

	// Remove from disk
	indexPath := filepath.Join(e.dataDir, name)
	if err := os.RemoveAll(indexPath); err != nil {
		return fmt.Errorf("failed to remove index directory %s: %w", indexPath, err)
	}

	log.Printf("Index '%s' deleted successfully.", name)
	return nil
}

// RenameIndex renames an index.
func (e *Engine) RenameIndex(oldName, newName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if oldName == newName {
		return errors.NewSameNameError(oldName)
	}

	instance, exists := e.indexes[oldName]
	if !exists {
		return errors.NewIndexNotFoundError(oldName)
	}

	if _, exists := e.indexes[newName]; exists {
		return errors.NewIndexAlreadyExistsError(newName)
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

	log.Printf("Index renamed from '%s' to '%s' successfully.", oldName, newName)
	return nil
}

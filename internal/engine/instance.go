package engine

import (
	"fmt"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/indexing"
	"github.com/gcbaptista/go-search-engine/internal/search"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gcbaptista/go-search-engine/store"
)

// IndexInstance holds all components and services for a single search index.
// It implements the services.IndexAccessor interface.
type IndexInstance struct {
	settings      *config.IndexSettings
	InvertedIndex *index.InvertedIndex
	DocumentStore *store.DocumentStore
	indexer       *indexing.Service
	searcher      *search.Service
}

// NewIndexInstance creates and initializes a new IndexInstance.
func NewIndexInstance(settings config.IndexSettings) (*IndexInstance, error) {
	if settings.Name == "" {
		return nil, fmt.Errorf("index name cannot be empty in settings")
	}

	docStore := &store.DocumentStore{
		Docs:                   make(map[uint32]model.Document),
		ExternalIDtoInternalID: make(map[string]uint32),
		NextID:                 0, // Start internal IDs from 0
	}

	invIndex := &index.InvertedIndex{
		Index:    make(map[string]index.PostingList),
		Settings: &settings,
	}

	indexerService, err := indexing.NewService(invIndex, docStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer service: %w", err)
	}

	return &IndexInstance{
		settings:      &settings,
		InvertedIndex: invIndex,
		DocumentStore: docStore,
		indexer:       indexerService,
		searcher:      nil, // Initialize searcher later to avoid circular dependencies
	}, nil
}

// AddDocuments delegates to the underlying Indexer service.
// This satisfies a part of the services.IndexAccessor interface.
func (i *IndexInstance) AddDocuments(docs []model.Document) error {
	if i.indexer == nil {
		return fmt.Errorf("indexer service not initialized for index '%s'", i.settings.Name)
	}
	return i.indexer.AddDocuments(docs)
}

// DeleteAllDocuments delegates to the underlying Indexer service.
// This satisfies a part of the services.IndexAccessor interface.
func (i *IndexInstance) DeleteAllDocuments() error {
	if i.indexer == nil {
		return fmt.Errorf("indexer service not initialized for index '%s'", i.settings.Name)
	}
	return i.indexer.DeleteAllDocuments()
}

// DeleteDocument delegates to the underlying Indexer service.
// This satisfies a part of the services.IndexAccessor interface.
func (i *IndexInstance) DeleteDocument(docID string) error {
	if i.indexer == nil {
		return fmt.Errorf("indexer service not initialized for index '%s'", i.settings.Name)
	}
	return i.indexer.DeleteDocument(docID)
}

// Search delegates to the underlying Searcher service.
// This satisfies a part of the services.IndexAccessor interface.
func (i *IndexInstance) Search(query services.SearchQuery) (services.SearchResult, error) {
	if i.searcher == nil {
		return services.SearchResult{}, fmt.Errorf("search service not initialized for index '%s'", i.settings.Name)
	}
	return i.searcher.Search(query)
}

// Settings returns the configuration settings for this index.
// This satisfies a part of the services.IndexAccessor interface.
func (i *IndexInstance) Settings() config.IndexSettings {
	return *i.settings
}

// SetSearcher allows late initialization of the searcher.
// This is a helper, typically called by the engine once search service is available.
func (i *IndexInstance) SetSearcher(searcher *search.Service) {
	i.searcher = searcher
}

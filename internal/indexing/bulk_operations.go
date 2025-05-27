package indexing

import (
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/model"
)

// BulkIndexingConfig contains configuration for bulk indexing operations
type BulkIndexingConfig struct {
	BatchSize         int           // Number of documents to process in each batch
	WorkerCount       int           // Number of parallel workers for processing
	FlushInterval     time.Duration // How often to flush accumulated changes
	MemoryThreshold   int           // Memory threshold in MB before forcing flush
	ProgressCallback  func(processed, total int, message string)
	EnableCompression bool // Whether to compress intermediate data
	OptimizeForMemory bool // Trade CPU for memory efficiency
}

// DefaultBulkIndexingConfig returns sensible defaults for bulk indexing
func DefaultBulkIndexingConfig() BulkIndexingConfig {
	return BulkIndexingConfig{
		BatchSize:         1000,             // Larger batches for efficiency
		WorkerCount:       runtime.NumCPU(), // Use all available cores
		FlushInterval:     5 * time.Second,  // Flush every 5 seconds
		MemoryThreshold:   500,              // 500MB threshold
		EnableCompression: false,            // Disabled by default for speed
		OptimizeForMemory: false,            // Optimize for speed by default
	}
}

// BulkIndexer provides high-performance bulk indexing operations
type BulkIndexer struct {
	service         *Service
	config          BulkIndexingConfig
	pendingUpdates  map[string][]index.PostingEntry // Token -> pending entries
	pendingDocs     map[uint32]model.Document       // Pending document updates
	pendingMappings map[string]uint32               // Pending ID mappings
	mu              sync.RWMutex
	lastFlush       time.Time
	processedCount  int
	totalCount      int
}

// NewBulkIndexer creates a new bulk indexer with the given configuration
func NewBulkIndexer(service *Service, config BulkIndexingConfig) *BulkIndexer {
	return &BulkIndexer{
		service:         service,
		config:          config,
		pendingUpdates:  make(map[string][]index.PostingEntry),
		pendingDocs:     make(map[uint32]model.Document),
		pendingMappings: make(map[string]uint32),
		lastFlush:       time.Now(),
	}
}

// BulkAddDocuments efficiently adds a large number of documents using parallel processing
func (bi *BulkIndexer) BulkAddDocuments(docs []model.Document) error {
	bi.totalCount = len(docs)
	bi.processedCount = 0

	if len(docs) == 0 {
		return nil
	}

	log.Printf("Starting bulk indexing of %d documents with %d workers", len(docs), bi.config.WorkerCount)
	start := time.Now()

	// Create worker pool
	docChan := make(chan []model.Document, bi.config.WorkerCount*2)
	resultChan := make(chan *bulkProcessResult, bi.config.WorkerCount*2)
	errorChan := make(chan error, bi.config.WorkerCount)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < bi.config.WorkerCount; i++ {
		wg.Add(1)
		go bi.worker(docChan, resultChan, errorChan, &wg)
	}

	// Start result collector
	collectorDone := make(chan struct{})
	go bi.resultCollector(resultChan, collectorDone)

	// Send work to workers in batches
	go func() {
		defer close(docChan)
		for i := 0; i < len(docs); i += bi.config.BatchSize {
			end := i + bi.config.BatchSize
			if end > len(docs) {
				end = len(docs)
			}
			batch := docs[i:end]
			docChan <- batch
		}
	}()

	// Wait for workers to complete
	wg.Wait()
	close(resultChan)

	// Wait for result collector to finish
	<-collectorDone

	// Check for errors
	select {
	case err := <-errorChan:
		return fmt.Errorf("bulk indexing failed: %w", err)
	default:
	}

	// Final flush
	if err := bi.flush(); err != nil {
		return fmt.Errorf("final flush failed: %w", err)
	}

	duration := time.Since(start)
	log.Printf("Bulk indexing completed: %d documents in %v (%.2f docs/sec)",
		len(docs), duration, float64(len(docs))/duration.Seconds())

	return nil
}

// bulkProcessResult contains the result of processing a batch of documents
type bulkProcessResult struct {
	tokenUpdates map[string][]index.PostingEntry
	docUpdates   map[uint32]model.Document
	idMappings   map[string]uint32
	processed    int
}

// worker processes batches of documents in parallel
func (bi *BulkIndexer) worker(docChan <-chan []model.Document, resultChan chan<- *bulkProcessResult, errorChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for batch := range docChan {
		result, err := bi.processBatch(batch)
		if err != nil {
			select {
			case errorChan <- err:
			default:
			}
			return
		}
		resultChan <- result
	}
}

// processBatch processes a batch of documents and returns the updates
func (bi *BulkIndexer) processBatch(docs []model.Document) (*bulkProcessResult, error) {
	result := &bulkProcessResult{
		tokenUpdates: make(map[string][]index.PostingEntry),
		docUpdates:   make(map[uint32]model.Document),
		idMappings:   make(map[string]uint32),
		processed:    len(docs),
	}

	settings := bi.service.invertedIndex.Settings

	// Pre-allocate internal IDs for this batch to avoid contention
	bi.service.documentStore.Mu.Lock()
	nextID := bi.service.documentStore.NextID
	batchIDMappings := make(map[string]uint32, len(docs))

	for _, doc := range docs {
		docIDValue, exists := doc["documentID"]
		if !exists {
			bi.service.documentStore.Mu.Unlock()
			return nil, fmt.Errorf("document missing documentID")
		}

		docIDStr, ok := docIDValue.(string)
		if !ok {
			bi.service.documentStore.Mu.Unlock()
			return nil, fmt.Errorf("documentID must be string")
		}

		if strings.TrimSpace(docIDStr) == "" {
			bi.service.documentStore.Mu.Unlock()
			return nil, fmt.Errorf("documentID cannot be empty")
		}

		docIDStr = strings.TrimSpace(docIDStr)

		// Check if document already exists
		if existingID, exists := bi.service.documentStore.ExternalIDtoInternalID[docIDStr]; exists {
			batchIDMappings[docIDStr] = existingID
		} else {
			batchIDMappings[docIDStr] = nextID
			nextID++
		}
	}

	bi.service.documentStore.NextID = nextID
	bi.service.documentStore.Mu.Unlock()

	// Process documents without holding locks
	for _, doc := range docs {
		docIDStr := strings.TrimSpace(doc["documentID"].(string))
		internalID := batchIDMappings[docIDStr]

		result.docUpdates[internalID] = doc
		result.idMappings[docIDStr] = internalID

		// Process each searchable field
		for _, fieldName := range settings.SearchableFields {
			fieldVal, fieldExists := doc[fieldName]
			if !fieldExists {
				continue
			}

			textContent := bi.extractTextContent(fieldVal)
			if strings.TrimSpace(textContent) == "" {
				continue
			}

			tokens := generateTokensForField(textContent, fieldName, settings)
			if len(tokens) == 0 {
				continue
			}

			// Calculate term frequencies
			termFrequencies := make(map[string]int)
			for _, token := range tokens {
				termFrequencies[token]++
			}

			// Create posting entries for each unique token
			for token, freq := range termFrequencies {
				entry := index.PostingEntry{
					DocID:     internalID,
					FieldName: fieldName,
					Score:     float64(freq),
				}
				result.tokenUpdates[token] = append(result.tokenUpdates[token], entry)
			}
		}
	}

	return result, nil
}

// resultCollector collects results from workers and accumulates them
func (bi *BulkIndexer) resultCollector(resultChan <-chan *bulkProcessResult, done chan<- struct{}) {
	defer close(done)

	for result := range resultChan {
		bi.mu.Lock()

		// Accumulate token updates
		for token, entries := range result.tokenUpdates {
			bi.pendingUpdates[token] = append(bi.pendingUpdates[token], entries...)
		}

		// Accumulate document updates
		for id, doc := range result.docUpdates {
			bi.pendingDocs[id] = doc
		}

		// Accumulate ID mappings
		for extID, intID := range result.idMappings {
			bi.pendingMappings[extID] = intID
		}

		bi.processedCount += result.processed
		bi.mu.Unlock()

		// Report progress
		if bi.config.ProgressCallback != nil {
			bi.config.ProgressCallback(bi.processedCount, bi.totalCount,
				fmt.Sprintf("Processed %d/%d documents", bi.processedCount, bi.totalCount))
		}

		// Check if we should flush
		bi.mu.RLock()
		shouldFlush := time.Since(bi.lastFlush) > bi.config.FlushInterval ||
			bi.estimateMemoryUsage() > bi.config.MemoryThreshold*1024*1024
		bi.mu.RUnlock()

		if shouldFlush {
			if err := bi.flush(); err != nil {
				log.Printf("Error during flush: %v", err)
			}
		}
	}
}

// flush applies all pending updates to the actual index
func (bi *BulkIndexer) flush() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if len(bi.pendingUpdates) == 0 && len(bi.pendingDocs) == 0 {
		return nil
	}

	log.Printf("Flushing %d token updates and %d document updates",
		len(bi.pendingUpdates), len(bi.pendingDocs))

	// Acquire locks for the flush operation
	bi.service.documentStore.Mu.Lock()
	bi.service.invertedIndex.Mu.Lock()
	defer bi.service.documentStore.Mu.Unlock()
	defer bi.service.invertedIndex.Mu.Unlock()

	// Apply document updates
	for id, doc := range bi.pendingDocs {
		bi.service.documentStore.Docs[id] = doc
	}

	// Apply ID mappings
	for extID, intID := range bi.pendingMappings {
		bi.service.documentStore.ExternalIDtoInternalID[extID] = intID
	}

	// Apply token updates efficiently
	for token, newEntries := range bi.pendingUpdates {
		currentList := bi.service.invertedIndex.Index[token]

		// Merge and sort the posting list
		mergedList := bi.mergePostingLists(currentList, newEntries)
		bi.service.invertedIndex.Index[token] = mergedList
	}

	// Clear pending updates
	bi.pendingUpdates = make(map[string][]index.PostingEntry)
	bi.pendingDocs = make(map[uint32]model.Document)
	bi.pendingMappings = make(map[string]uint32)
	bi.lastFlush = time.Now()

	return nil
}

// mergePostingLists efficiently merges two posting lists while maintaining sort order
func (bi *BulkIndexer) mergePostingLists(existing, new []index.PostingEntry) index.PostingList {
	if len(existing) == 0 {
		// Sort new entries
		sort.Slice(new, func(i, j int) bool {
			if new[i].Score != new[j].Score {
				return new[i].Score > new[j].Score // Score descending
			}
			if new[i].DocID != new[j].DocID {
				return new[i].DocID < new[j].DocID // DocID ascending
			}
			return new[i].FieldName < new[j].FieldName // FieldName ascending
		})
		return new
	}

	if len(new) == 0 {
		return existing
	}

	// Create a map for efficient lookups and updates
	entryMap := make(map[string]index.PostingEntry)

	// Add existing entries
	for _, entry := range existing {
		key := fmt.Sprintf("%d:%s", entry.DocID, entry.FieldName)
		entryMap[key] = entry
	}

	// Add/update with new entries
	for _, entry := range new {
		key := fmt.Sprintf("%d:%s", entry.DocID, entry.FieldName)
		entryMap[key] = entry // This will overwrite existing entries for the same doc+field
	}

	// Convert back to slice and sort
	result := make(index.PostingList, 0, len(entryMap))
	for _, entry := range entryMap {
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score // Score descending
		}
		if result[i].DocID != result[j].DocID {
			return result[i].DocID < result[j].DocID // DocID ascending
		}
		return result[i].FieldName < result[j].FieldName // FieldName ascending
	})

	return result
}

// extractTextContent extracts text content from various field types
func (bi *BulkIndexer) extractTextContent(fieldVal interface{}) string {
	switch v := fieldVal.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if strItem, ok := item.(string); ok {
				parts = append(parts, strItem)
			}
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(v, " ")
	default:
		return ""
	}
}

// estimateMemoryUsage provides a rough estimate of memory usage for pending updates
func (bi *BulkIndexer) estimateMemoryUsage() int {
	// Rough estimation: each posting entry ~100 bytes, each document ~1KB
	tokenMemory := len(bi.pendingUpdates) * 100
	for _, entries := range bi.pendingUpdates {
		tokenMemory += len(entries) * 100
	}
	docMemory := len(bi.pendingDocs) * 1024
	return tokenMemory + docMemory
}

// BulkReindex performs an optimized reindexing operation
func (s *Service) BulkReindex(config BulkIndexingConfig) error {
	log.Printf("Starting bulk reindex operation")
	start := time.Now()

	// Extract all documents efficiently
	s.documentStore.Mu.RLock()
	docs := make([]model.Document, 0, len(s.documentStore.Docs))
	for _, doc := range s.documentStore.Docs {
		docs = append(docs, doc)
	}
	s.documentStore.Mu.RUnlock()

	if len(docs) == 0 {
		log.Printf("No documents to reindex")
		return nil
	}

	log.Printf("Extracted %d documents for reindexing", len(docs))

	// Clear the index efficiently
	s.documentStore.Mu.Lock()
	s.invertedIndex.Mu.Lock()
	s.documentStore.Docs = make(map[uint32]model.Document)
	s.documentStore.ExternalIDtoInternalID = make(map[string]uint32)
	s.documentStore.NextID = 0
	s.invertedIndex.Index = make(map[string]index.PostingList)
	s.documentStore.Mu.Unlock()
	s.invertedIndex.Mu.Unlock()

	// Use bulk indexer for efficient re-indexing
	bulkIndexer := NewBulkIndexer(s, config)
	if err := bulkIndexer.BulkAddDocuments(docs); err != nil {
		return fmt.Errorf("bulk reindex failed: %w", err)
	}

	duration := time.Since(start)
	log.Printf("Bulk reindex completed: %d documents in %v (%.2f docs/sec)",
		len(docs), duration, float64(len(docs))/duration.Seconds())

	return nil
}

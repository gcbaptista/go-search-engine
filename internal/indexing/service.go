package indexing

import (
	"fmt"
	"log"
	"time"

	"sort"
	"strings"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/tokenizer"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/store"
)

// Service implements the indexing logic for a single index.
// It fulfills the services.Indexer interface.
type Service struct {
	invertedIndex *index.InvertedIndex
	documentStore *store.DocumentStore
	// settings are accessible via invertedIndex.Settings
}

// NewService creates a new indexing Service.
// It assumes that invertedIndex and documentStore are properly initialized,
// and that invertedIndex.Settings is not nil.
func NewService(invertedIndex *index.InvertedIndex, documentStore *store.DocumentStore) (*Service, error) {
	if invertedIndex == nil {
		return nil, fmt.Errorf("inverted index cannot be nil")
	}
	if documentStore == nil {
		return nil, fmt.Errorf("document store cannot be nil")
	}
	if invertedIndex.Index == nil {
		// Initialize the map if it's nil to prevent panics later
		invertedIndex.Index = make(map[string]index.PostingList)
	}
	if documentStore.Docs == nil {
		documentStore.Docs = make(map[uint32]model.Document)
	}
	if documentStore.ExternalIDtoInternalID == nil {
		documentStore.ExternalIDtoInternalID = make(map[string]uint32)
	}
	if invertedIndex.Settings == nil {
		return nil, fmt.Errorf("inverted index settings cannot be nil")
	}
	return &Service{
		invertedIndex: invertedIndex,
		documentStore: documentStore,
	}, nil
}

// AddDocuments adds a batch of documents to the index.
// This satisfies the services.Indexer interface.
func (s *Service) AddDocuments(docs []model.Document) error {
	// Process documents in micro-batches to minimize lock contention and allow search operations to interleave
	const microBatchSize = 10 // Very small batches to minimize lock hold time

	for i := 0; i < len(docs); i += microBatchSize {
		end := i + microBatchSize
		if end > len(docs) {
			end = len(docs)
		}

		microBatch := docs[i:end]
		if err := s.addDocumentMicroBatch(microBatch); err != nil {
			return fmt.Errorf("failed to add document micro-batch starting at index %d: %w", i, err)
		}

		// Yield to allow search operations to proceed between micro-batches
		// This prevents search starvation during large indexing operations
		if i+microBatchSize < len(docs) {
			// Small delay to allow pending read operations to acquire locks
			// This is a cooperative scheduling approach
			time.Sleep(1 * time.Millisecond)
		}
	}
	return nil
}

// addDocumentMicroBatch processes a very small batch of documents with minimal lock time
func (s *Service) addDocumentMicroBatch(docs []model.Document) error {
	// Acquire locks for the micro-batch operation
	s.documentStore.Mu.Lock()
	s.invertedIndex.Mu.Lock()
	defer s.documentStore.Mu.Unlock()
	defer s.invertedIndex.Mu.Unlock()

	for _, doc := range docs {
		// Extract documentID string from doc map for error reporting if addSingleDocumentUnsafe fails
		// This is a bit redundant with the extraction inside addSingleDocumentUnsafe, but useful for top-level error context.
		docIDForErrorReporting := "<unknown>"
		if idVal, ok := doc["documentID"]; ok {
			if idStr, isStr := idVal.(string); isStr {
				docIDForErrorReporting = idStr
			}
		}
		if err := s.addSingleDocumentUnsafe(doc); err != nil {
			// Return on first error
			return fmt.Errorf("failed to add document ID %s: %w", docIDForErrorReporting, err)
		}
	}
	return nil
}

// addSingleDocumentUnsafe handles the processing and indexing of a single document.
// It assumes that the caller already holds locks on documentStore and invertedIndex.
func (s *Service) addSingleDocumentUnsafe(doc model.Document) error {
	// Attempt to get documentID from the document map, or expect it to be handled by API layer.
	// For DocumentStore, the documentID is the external ID.
	docIDValue, docIDExists := doc["documentID"] // Check if "documentID" key exists
	var docIDStr string
	if docIDExists {
		switch v := docIDValue.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("document documentID cannot be empty or whitespace-only")
			}
			docIDStr = strings.TrimSpace(v)
		default:
			return fmt.Errorf("document documentID has an invalid type in the map, expected string")
		}
	} else {
		return fmt.Errorf("document documentID not found in document map or is nil; documentID must be provided in the document data with key 'documentID'")
	}

	settings := s.invertedIndex.Settings
	var oldDoc model.Document
	isUpdate := false

	// 1. Get/Assign Internal ID
	internalID, exists := s.documentStore.ExternalIDtoInternalID[docIDStr]
	if exists {
		isUpdate = true
		// It's an update, retrieve the old document for cleanup
		if doc, ok := s.documentStore.Docs[internalID]; ok {
			oldDoc = doc
		} else {
			// This case should ideally not happen if ExternalIDtoInternalID and Docs are consistent
			// If it does, we can't clean up old tokens effectively based on old content.
			// Proceeding will just overwrite/add new tokens.
			log.Printf("Warning: Document with internalID %d found in ExternalIDtoInternalID but not in Docs. Cannot clean up old tokens for documentID %s.\n", internalID, docIDStr)
		}
	} else {
		internalID = s.documentStore.NextID
		s.documentStore.ExternalIDtoInternalID[docIDStr] = internalID
		s.documentStore.NextID++
	}

	// 2. If it's an update and we have the old document, clean up its old tokens
	if isUpdate && oldDoc != nil {
		for _, fieldName := range settings.SearchableFields {
			if oldFieldVal, fieldExists := oldDoc[fieldName]; fieldExists {
				var oldTextContent string
				switch v := oldFieldVal.(type) {
				case string:
					oldTextContent = v
				case []interface{}:
					var parts []string
					for _, item := range v {
						if strItem, ok := item.(string); ok {
							parts = append(parts, strItem)
						}
					}
					oldTextContent = strings.Join(parts, " ")
				case []string:
					oldTextContent = strings.Join(v, " ")
				default:
					// If type was unhandled before, it won't be indexed, so no cleanup needed for this field
					continue
				}

				if strings.TrimSpace(oldTextContent) == "" {
					continue
				}

				oldTokens := generateTokensForField(oldTextContent, fieldName, settings)
				uniqueOldTokens := make(map[string]struct{})
				for _, token := range oldTokens {
					uniqueOldTokens[token] = struct{}{}
				}

				for oldToken := range uniqueOldTokens {
					if postingList, ok := s.invertedIndex.Index[oldToken]; ok {
						newList := make(index.PostingList, 0, len(postingList))
						for _, entry := range postingList {
							if entry.DocID != internalID || entry.FieldName != fieldName {
								newList = append(newList, entry)
							}
						}
						if len(newList) == 0 {
							delete(s.invertedIndex.Index, oldToken)
						} else {
							s.invertedIndex.Index[oldToken] = newList
						}
					}
				}
			}
		}
	}

	// Store/Update the full document in the document store *after* potential cleanup based on its old version
	s.documentStore.Docs[internalID] = doc

	// 3. Process searchable fields specified in index settings for the new/updated document
	for _, fieldName := range settings.SearchableFields {
		fieldVal, fieldExists := doc[fieldName]
		if !fieldExists {
			log.Printf("Warning: Searchable field '%s' not found in document documentID '%s'.\n", fieldName, docIDStr)
			continue
		}

		var textContent string
		switch v := fieldVal.(type) {
		case string:
			textContent = v
		case []interface{}: // JSON arrays are often unmarshalled to []interface{}
			var parts []string
			for _, item := range v {
				if strItem, ok := item.(string); ok {
					parts = append(parts, strItem)
				}
			}
			textContent = strings.Join(parts, " ")
		case []string: // If it was explicitly a []string
			textContent = strings.Join(v, " ")
		default:
			log.Printf("Warning: Searchable field '%s' in document documentID '%s' has unhandled type %T.\n", fieldName, docIDStr, fieldVal)
			continue
		}

		if strings.TrimSpace(textContent) == "" {
			continue // Skip if the field yields no text content
		}

		tokens := generateTokensForField(textContent, fieldName, settings)

		if len(tokens) == 0 {
			continue // Skip if tokenization yields no tokens
		}

		// Calculate term frequencies for the current document's content
		termFrequencies := make(map[string]int)
		for _, token := range tokens {
			termFrequencies[token]++
		}

		// 4. Update Inverted Index for each unique token with its frequency in this field
		for token, freqInField := range termFrequencies {
			newPostingEntry := index.PostingEntry{
				DocID:     internalID,
				FieldName: fieldName,            // Store the field name
				Score:     float64(freqInField), // Term frequency within this specific field
			}

			currentPostingList := s.invertedIndex.Index[token]

			// Check if an entry for this DocID and FieldName already exists for this token.
			// This is important if re-indexing or if a document update occurs.
			existingIdx := -1
			for i, entry := range currentPostingList {
				if entry.DocID == internalID && entry.FieldName == fieldName {
					existingIdx = i
					break
				}
			}

			if existingIdx != -1 {
				// Remove existing entry to re-insert the updated one.
				currentPostingList = append(currentPostingList[:existingIdx], currentPostingList[existingIdx+1:]...)
			}

			// Find the correct insertion point to keep the list sorted by Score (descending),
			// then by DocID (ascending), then by FieldName (ascending).
			insertionIdx := sort.Search(len(currentPostingList), func(i int) bool {
				if currentPostingList[i].Score != newPostingEntry.Score {
					return currentPostingList[i].Score < newPostingEntry.Score // Sort by Score descending
				}
				if currentPostingList[i].DocID != newPostingEntry.DocID {
					return currentPostingList[i].DocID > newPostingEntry.DocID // Sort by DocID ascending
				}
				return currentPostingList[i].FieldName >= newPostingEntry.FieldName // Sort by FieldName ascending
			})

			currentPostingList = append(currentPostingList, index.PostingEntry{})        // Allocate space
			copy(currentPostingList[insertionIdx+1:], currentPostingList[insertionIdx:]) // Shift elements
			currentPostingList[insertionIdx] = newPostingEntry                           // Insert
			s.invertedIndex.Index[token] = currentPostingList
		}
	}
	return nil
}

// generateTokensForField decides whether to use n-grams based on field-specific settings.
func generateTokensForField(text string, fieldName string, settings *config.IndexSettings) []string {
	// Check if the current field is in the list of fields where prefix search should be disabled
	for _, noPrefixField := range settings.FieldsWithoutPrefixSearch {
		if fieldName == noPrefixField {
			return tokenizer.Tokenize(text) // Use regular tokenization (whole words)
		}
	}

	// If not disabled for this field, use prefix n-grams
	return tokenizer.TokenizeWithPrefixNGrams(text)
}

// DeleteAllDocuments removes all documents from the index, clearing both the document store and inverted index.
// This satisfies the services.Indexer interface.
func (s *Service) DeleteAllDocuments() error {
	// Acquire locks for the entire operation
	s.documentStore.Mu.Lock()
	s.invertedIndex.Mu.Lock()
	defer s.documentStore.Mu.Unlock()
	defer s.invertedIndex.Mu.Unlock()

	// Clear the document store
	s.documentStore.Docs = make(map[uint32]model.Document)
	s.documentStore.ExternalIDtoInternalID = make(map[string]uint32)
	s.documentStore.NextID = 0

	// Clear the inverted index
	s.invertedIndex.Index = make(map[string]index.PostingList)

	return nil
}

// DeleteDocument removes a specific document from the index by its external ID.
// This satisfies the services.Indexer interface.
func (s *Service) DeleteDocument(docID string) error {
	// Acquire locks for the entire operation
	s.documentStore.Mu.Lock()
	s.invertedIndex.Mu.Lock()
	defer s.documentStore.Mu.Unlock()
	defer s.invertedIndex.Mu.Unlock()

	// Check if the document exists
	internalID, exists := s.documentStore.ExternalIDtoInternalID[docID]
	if !exists {
		return fmt.Errorf("document with ID '%s' not found", docID)
	}

	// Get the document to clean up its tokens
	doc, docExists := s.documentStore.Docs[internalID]
	if !docExists {
		// This case should ideally not happen if ExternalIDtoInternalID and Docs are consistent
		// Clean up the mapping and return error
		delete(s.documentStore.ExternalIDtoInternalID, docID)
		return fmt.Errorf("document with ID '%s' found in mapping but not in store (inconsistent state)", docID)
	}

	settings := s.invertedIndex.Settings

	// Remove tokens from inverted index for each searchable field
	for _, fieldName := range settings.SearchableFields {
		if fieldVal, fieldExists := doc[fieldName]; fieldExists {
			var textContent string
			switch v := fieldVal.(type) {
			case string:
				textContent = v
			case []interface{}:
				var parts []string
				for _, item := range v {
					if strItem, ok := item.(string); ok {
						parts = append(parts, strItem)
					}
				}
				textContent = strings.Join(parts, " ")
			case []string:
				textContent = strings.Join(v, " ")
			default:
				// If type was unhandled, it won't be indexed, so no cleanup needed for this field
				continue
			}

			if strings.TrimSpace(textContent) == "" {
				continue
			}

			// Generate tokens for this field
			tokens := generateTokensForField(textContent, fieldName, settings)
			uniqueTokens := make(map[string]struct{})
			for _, token := range tokens {
				uniqueTokens[token] = struct{}{}
			}

			// Remove document from posting lists for each token
			for token := range uniqueTokens {
				if postingList, ok := s.invertedIndex.Index[token]; ok {
					newList := make(index.PostingList, 0, len(postingList))
					for _, entry := range postingList {
						// Keep entries that don't match this document and field
						if entry.DocID != internalID || entry.FieldName != fieldName {
							newList = append(newList, entry)
						}
					}
					// If no entries remain for this token, remove the token entirely
					if len(newList) == 0 {
						delete(s.invertedIndex.Index, token)
					} else {
						s.invertedIndex.Index[token] = newList
					}
				}
			}
		}
	}

	// Remove document from document store
	delete(s.documentStore.Docs, internalID)
	delete(s.documentStore.ExternalIDtoInternalID, docID)

	return nil
}

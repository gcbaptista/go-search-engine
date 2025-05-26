package store

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/gcbaptista/go-search-engine/model"
)

func init() {
	// Register common types that might appear in model.Document (map[string]interface{})
	// This helps Gob know how to handle them when they are stored as interface{} values.
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
	// Add other common primitive types or simple structs if they appear directly in maps.
	// For slices of specific types like []string, if they are consistently used, register them.
	// However, json.Unmarshal into map[string]interface{} often gives []interface{} for arrays.
	gob.Register([]string{})
	gob.Register(float64(0))
	gob.Register(false)
}

type DocumentStore struct {
	Mu                     sync.RWMutex
	Docs                   map[uint32]model.Document // Internal ID to full document
	ExternalIDtoInternalID map[string]uint32         // User-provided ID to internal uint32 ID
	NextID                 uint32
}

// gobDocumentStoreData is a helper struct for Gob encoding/decoding DocumentStore data.
// It excludes the mutex.
type gobDocumentStoreData struct {
	Docs                   map[uint32]model.Document
	ExternalIDtoInternalID map[string]uint32
	NextID                 uint32
}

// GobEncode implements the gob.GobEncoder interface for DocumentStore.
func (ds *DocumentStore) GobEncode() ([]byte, error) {
	ds.Mu.RLock()
	defer ds.Mu.RUnlock()

	// Create a deep copy of Docs to modify for Gob encoding if necessary
	// This is to handle potential []interface{} from JSON unmarshalling
	storableDocs := make(map[uint32]model.Document, len(ds.Docs))
	for id, doc := range ds.Docs {
		storableDoc := make(model.Document, len(doc))
		for k, val := range doc {
			if interfaceSlice, ok := val.([]interface{}); ok {
				// Attempt to convert []interface{} to []string if all elements are strings
				stringSlice := make([]string, 0, len(interfaceSlice))
				canConvertToStringSlice := true
				for _, item := range interfaceSlice {
					if strItem, isString := item.(string); isString {
						stringSlice = append(stringSlice, strItem)
					} else {
						canConvertToStringSlice = false
						break
					}
				}
				if canConvertToStringSlice {
					storableDoc[k] = stringSlice // Store as []string
				} else {
					storableDoc[k] = val // Store as is, relying on gob.Register
				}
			} else {
				storableDoc[k] = val
			}
		}
		storableDocs[id] = storableDoc
	}

	dataToEncode := gobDocumentStoreData{
		Docs:                   storableDocs, // Use the modified docs
		ExternalIDtoInternalID: ds.ExternalIDtoInternalID,
		NextID:                 ds.NextID,
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(dataToEncode); err != nil {
		return nil, fmt.Errorf("failed to gob encode document store data: %w", err)
	}
	return buf.Bytes(), nil
}

// GobDecode implements the gob.GobDecoder interface for DocumentStore.
func (ds *DocumentStore) GobDecode(data []byte) error {
	decodedData := gobDocumentStoreData{}

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&decodedData); err != nil {
		return fmt.Errorf("failed to gob decode document store data: %w", err)
	}

	ds.Mu.Lock()
	defer ds.Mu.Unlock()

	ds.Docs = decodedData.Docs
	ds.ExternalIDtoInternalID = decodedData.ExternalIDtoInternalID
	ds.NextID = decodedData.NextID

	// Ensure maps are initialized if they were nil after decoding
	if ds.Docs == nil {
		ds.Docs = make(map[uint32]model.Document)
	}
	// After decoding, []string might be present. If the application logic strictly expects
	// []interface{} for such fields in-memory post-load, a reverse conversion might be needed here.
	// For now, we assume that having []string for previously []interface{}-containing-only-strings is acceptable.

	if ds.ExternalIDtoInternalID == nil {
		ds.ExternalIDtoInternalID = make(map[string]uint32)
	}

	return nil
}

package index

import (
	"bytes"
	"encoding/gob"
	"sync"

	"github.com/gcbaptista/go-search-engine/config"
)

// InvertedIndex maps a term (token) to a list of documents containing that term,
// sorted by their ranking score (popularity).
type InvertedIndex struct {
	Mu       sync.RWMutex
	Index    map[string]PostingList
	Settings *config.IndexSettings // Reference to settings for this index
}

// gobInvertedIndexData is a helper struct for Gob encoding/decoding InvertedIndex data.
// It excludes the mutex.
type gobInvertedIndexData struct {
	Index    map[string]PostingList
	Settings *config.IndexSettings
}

// GobEncode implements the gob.GobEncoder interface for InvertedIndex.
func (ii *InvertedIndex) GobEncode() ([]byte, error) {
	ii.Mu.RLock() // Ensure consistent data during encoding
	defer ii.Mu.RUnlock()

	dataToEncode := gobInvertedIndexData{
		Index:    ii.Index,
		Settings: ii.Settings,
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(dataToEncode); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements the gob.GobDecoder interface for InvertedIndex.
func (ii *InvertedIndex) GobDecode(data []byte) error {
	decodedData := gobInvertedIndexData{}

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&decodedData); err != nil {
		return err
	}

	ii.Mu.Lock() // Ensure exclusive access during decoding
	defer ii.Mu.Unlock()

	ii.Index = decodedData.Index
	ii.Settings = decodedData.Settings

	// Ensure maps are initialized if they were nil after decoding (e.g. from an empty file)
	if ii.Index == nil {
		ii.Index = make(map[string]PostingList)
	}

	// Settings can be nil if not present, no need to force initialize unless required by logic
	return nil
}

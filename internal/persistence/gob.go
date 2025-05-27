package persistence

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

// SaveGob encodes the given object using gob and saves it to the specified filePath.
// It creates necessary directories if they don't exist.
func SaveGob(filePath string, object interface{}) error {
	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.Create(filePath) // #nosec G304 -- filePath is controlled by application, not user input
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't override the main error
			fmt.Printf("Warning: failed to close file %s: %v\n", filePath, closeErr)
		}
	}()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(object); err != nil {
		return fmt.Errorf("failed to gob encode to file %s: %w", filePath, err)
	}
	return nil
}

// LoadGob decodes a gob-encoded file from filePath into the provided object pointer.
// The object must be a pointer to the type that was originally encoded.
// If the file does not exist, it returns os.ErrNotExist, allowing callers to handle
// fresh starts gracefully.
func LoadGob(filePath string, objectPointer interface{}) error {
	file, err := os.Open(filePath) // #nosec G304 -- filePath is controlled by application, not user input
	if err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist // Return specific error for non-existent file
		}
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't override the main error
			fmt.Printf("Warning: failed to close file %s: %v\n", filePath, closeErr)
		}
	}()

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(objectPointer); err != nil {
		return fmt.Errorf("failed to gob decode from file %s: %w", filePath, err)
	}
	return nil
}

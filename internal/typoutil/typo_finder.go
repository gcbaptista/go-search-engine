package typoutil

import (
	"log"
	"sync"
	"time"
)

// TypoFinder provides typo tolerance functionality with caching and time limits
type TypoFinder struct {
	// Precomputed list of all indexed terms (updated when index changes)
	indexedTerms []string

	// Optional: Cache for frequently requested typos
	// Key: term + maxDistance, Value: slice of typos
	cache   map[string][]string
	cacheMu sync.RWMutex

	// Cache size limit to prevent memory bloat
	maxCacheSize int
}

// NewTypoFinder creates a new typo finder with caching
func NewTypoFinder(indexedTerms []string) *TypoFinder {
	return &TypoFinder{
		indexedTerms: make([]string, len(indexedTerms)),
		cache:        make(map[string][]string),
		maxCacheSize: 1000, // Limit cache to 1000 entries
	}
}

// UpdateIndexedTerms updates the list of indexed terms (call when index changes)
func (tf *TypoFinder) UpdateIndexedTerms(indexedTerms []string) {
	tf.indexedTerms = make([]string, len(indexedTerms))
	copy(tf.indexedTerms, indexedTerms)

	// Clear cache as it's now invalid
	tf.cacheMu.Lock()
	tf.cache = make(map[string][]string)
	tf.cacheMu.Unlock()
}

// GenerateTypos finds typos with caching and time limits
func (tf *TypoFinder) GenerateTypos(term string, maxDistance int, maxResults int) []string {
	return tf.GenerateTyposWithTimeLimit(term, maxDistance, maxResults, 50*time.Millisecond)
}

// GenerateTyposWithTimeLimit finds typos with dual criteria: max results OR time limit
func (tf *TypoFinder) GenerateTyposWithTimeLimit(term string, maxDistance int, maxResults int, timeLimit time.Duration) []string {
	if maxDistance <= 0 || term == "" || len(tf.indexedTerms) == 0 {
		return []string{}
	}

	// Check cache first
	cacheKey := term + string(rune(maxDistance))
	tf.cacheMu.RLock()
	if cached, exists := tf.cache[cacheKey]; exists {
		tf.cacheMu.RUnlock()
		if maxResults > 0 && len(cached) > maxResults {
			return cached[:maxResults]
		}
		return cached
	}
	tf.cacheMu.RUnlock()

	typos := tf.findTyposWithDualCriteria(term, maxDistance, maxResults, timeLimit)

	// Cache result if cache isn't too large
	tf.cacheMu.Lock()
	if len(tf.cache) < tf.maxCacheSize {
		tf.cache[cacheKey] = typos
	}
	tf.cacheMu.Unlock()

	return typos
}

// findTyposWithDualCriteria implements the core typo finding with dual stopping criteria
func (tf *TypoFinder) findTyposWithDualCriteria(term string, maxDistance int, maxResults int, timeLimit time.Duration) []string {
	termLen := len([]rune(term))
	typos := make([]string, 0, maxResults) // Pre-allocate with expected size
	startTime := time.Now()

	for i, indexedTerm := range tf.indexedTerms {
		// Check time limit first (most important criterion)
		if time.Since(startTime) >= timeLimit {
			// Log warning if we haven't reached the target and there are more terms to check
			remainingTerms := len(tf.indexedTerms) - i
			if len(typos) < maxResults && remainingTerms > 0 {
				log.Printf("Warning: Typo search time limit reached (%.1fms) - found %d/%d tokens, %d terms remaining unchecked (term='%s', distance=%d)",
					float64(timeLimit.Nanoseconds())/1e6, len(typos), maxResults, remainingTerms, term, maxDistance)
			}
			break // Time limit reached
		}

		// Skip self
		if indexedTerm == term {
			continue
		}

		// Length-based early filtering: if length difference > maxDistance, skip
		indexedTermLen := len([]rune(indexedTerm))
		lengthDiff := indexedTermLen - termLen
		if lengthDiff < 0 {
			lengthDiff = -lengthDiff
		}
		if lengthDiff > maxDistance {
			continue
		}

		// Calculate actual Levenshtein distance
		dist := CalculateDamerauLevenshteinDistanceWithLimit(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance {
			typos = append(typos, indexedTerm)

			// Check if we've reached the result limit
			if maxResults > 0 && len(typos) >= maxResults {
				break // Result count limit reached
			}
		}
	}

	return typos
}

// findTyposWithLimits implements the core typo finding logic with result limits
func (tf *TypoFinder) findTyposWithLimits(term string, maxDistance int, maxResults int) []string {
	termLen := len([]rune(term))
	typos := make([]string, 0, maxResults) // Pre-allocate with expected size

	for _, indexedTerm := range tf.indexedTerms {
		// Skip self
		if indexedTerm == term {
			continue
		}

		// Length-based early filtering: if length difference > maxDistance, skip
		indexedTermLen := len([]rune(indexedTerm))
		lengthDiff := indexedTermLen - termLen
		if lengthDiff < 0 {
			lengthDiff = -lengthDiff
		}
		if lengthDiff > maxDistance {
			continue
		}

		// Calculate actual Levenshtein distance
		dist := CalculateDamerauLevenshteinDistanceWithLimit(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance {
			typos = append(typos, indexedTerm)

			// Early termination if we have enough results
			if maxResults > 0 && len(typos) >= maxResults {
				break
			}
		}
	}

	return typos
}

// GenerateTyposSimple provides a simple interface similar to the original function
// but with early termination and length filtering
func GenerateTyposSimple(term string, allIndexedTerms []string, maxDistance int) []string {
	if maxDistance <= 0 || term == "" || len(allIndexedTerms) == 0 {
		return []string{}
	}

	termLen := len([]rune(term))
	typos := make([]string, 0)

	for _, indexedTerm := range allIndexedTerms {
		if indexedTerm == term {
			continue
		}

		// Length-based early filtering
		indexedTermLen := len([]rune(indexedTerm))
		lengthDiff := indexedTermLen - termLen
		if lengthDiff < 0 {
			lengthDiff = -lengthDiff
		}
		if lengthDiff > maxDistance {
			continue
		}

		dist := CalculateDamerauLevenshteinDistanceWithLimit(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance {
			typos = append(typos, indexedTerm)
		}
	}

	return typos
}

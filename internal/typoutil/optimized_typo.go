package typoutil

import (
	"log"
	"sync"
	"time"
)

// TypoFinder provides optimized typo tolerance functionality
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

// NewTypoFinder creates a new optimized typo finder
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

// GenerateTypos finds typos with caching and optimizations
func (tf *TypoFinder) GenerateTypos(term string, maxDistance int, maxResults int) []string {
	return tf.GenerateTyposWithTimeLimit(term, maxDistance, maxResults, 50*time.Millisecond)
}

// GenerateTyposOptimized finds typos with multiple optimizations
func (tf *TypoFinder) GenerateTyposOptimized(term string, maxDistance int, maxResults int) []string {
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
		dist := CalculateLevenshteinDistanceOptimized(term, indexedTerm, maxDistance)
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

// findTyposWithOptimizations implements the core optimized typo finding logic
func (tf *TypoFinder) findTyposWithOptimizations(term string, maxDistance int, maxResults int) []string {
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
		dist := CalculateLevenshteinDistanceOptimized(term, indexedTerm, maxDistance)
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

// CalculateLevenshteinDistanceOptimized calculates Levenshtein distance with early termination
func CalculateLevenshteinDistanceOptimized(a, b string, maxDistance int) int {
	runesA := []rune(a)
	runesB := []rune(b)

	lenA := len(runesA)
	lenB := len(runesB)

	// Early termination: if length difference > maxDistance, return early
	lengthDiff := lenA - lenB
	if lengthDiff < 0 {
		lengthDiff = -lengthDiff
	}
	if lengthDiff > maxDistance {
		return maxDistance + 1 // Return a value > maxDistance to indicate no match
	}

	if lenA == 0 {
		return lenB
	}
	if lenB == 0 {
		return lenA
	}

	// Use two rows instead of full matrix to save memory (space optimization)
	prevRow := make([]int, lenB+1)
	currRow := make([]int, lenB+1)

	// Initialize first row
	for j := 0; j <= lenB; j++ {
		prevRow[j] = j
	}

	for i := 1; i <= lenA; i++ {
		currRow[0] = i
		minInRow := i // Track minimum value in current row for early termination

		for j := 1; j <= lenB; j++ {
			cost := 0
			if runesA[i-1] != runesB[j-1] {
				cost = 1
			}

			deletion := prevRow[j] + 1
			insertion := currRow[j-1] + 1
			substitution := prevRow[j-1] + cost

			currRow[j] = min(deletion, insertion, substitution)

			if currRow[j] < minInRow {
				minInRow = currRow[j]
			}
		}

		// Early termination: if minimum value in current row > maxDistance,
		// the final result will definitely be > maxDistance
		if minInRow > maxDistance {
			return maxDistance + 1
		}

		// Swap rows
		prevRow, currRow = currRow, prevRow
	}

	return prevRow[lenB]
}

// GenerateTyposSimple provides a simple interface similar to the original function
// but with optimizations
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

		dist := CalculateLevenshteinDistanceOptimized(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance {
			typos = append(typos, indexedTerm)
		}
	}

	return typos
}

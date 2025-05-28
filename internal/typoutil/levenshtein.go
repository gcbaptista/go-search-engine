package typoutil

// CalculateEditDistance calculates Damerau-Levenshtein distance with early termination
// This includes transposition operations in addition to insertion, deletion, and substitution
// Returns maxDistance + 1 if the actual distance exceeds maxDistance (for performance)
func CalculateEditDistance(a, b string, maxDistance int) int {
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

	// For Damerau-Levenshtein, we need three rows instead of two to handle transpositions
	// prevPrevRow: i-2 row (needed for transposition)
	// prevRow: i-1 row
	// currRow: i row (current)
	prevPrevRow := make([]int, lenB+1)
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

			// Standard operations: deletion, insertion, substitution
			deletion := prevRow[j] + 1
			insertion := currRow[j-1] + 1
			substitution := prevRow[j-1] + cost

			currRow[j] = min3(deletion, insertion, substitution)

			// Transposition operation (Damerau extension)
			// Check if we can do a transposition: characters at positions (i-1,j) and (i,j-1)
			// are swapped versions of characters at positions (i,j-1) and (i-1,j)
			if i > 1 && j > 1 &&
				runesA[i-1] == runesB[j-2] &&
				runesA[i-2] == runesB[j-1] {
				transposition := prevPrevRow[j-2] + cost
				if transposition < currRow[j] {
					currRow[j] = transposition
				}
			}

			if currRow[j] < minInRow {
				minInRow = currRow[j]
			}
		}

		// Early termination: if minimum value in current row > maxDistance,
		// the final result will definitely be > maxDistance
		if minInRow > maxDistance {
			return maxDistance + 1
		}

		// Rotate rows: prevPrevRow <- prevRow <- currRow
		prevPrevRow, prevRow, currRow = prevRow, currRow, prevPrevRow
	}

	return prevRow[lenB]
}

// min3 is a helper function to find the minimum of three integers
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// GenerateTypos finds terms from a list of indexedTerms that are within a given maxDistance (Damerau-Levenshtein) from the input term.
func GenerateTypos(term string, allIndexedTerms []string, maxDistance int) []string {
	typos := make([]string, 0) // Initialize as empty slice, not nil
	if maxDistance <= 0 || term == "" || len(allIndexedTerms) == 0 {
		return typos // Return empty slice instead of nil
	}

	termLen := len([]rune(term))

	for _, indexedTerm := range allIndexedTerms {
		// Avoid comparing the term to itself if it happens to be in allIndexedTerms already for typo generation
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

		dist := CalculateEditDistance(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance { // dist > 0 ensures it's a different word
			typos = append(typos, indexedTerm)
		}
	}
	return typos
}

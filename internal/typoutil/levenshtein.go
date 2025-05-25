package typoutil

// CalculateLevenshteinDistance computes the Levenshtein distance between two strings.
// It represents the minimum number of single-character edits (insertions, deletions, or substitutions)
// required to change one word into the other.
// This implementation properly handles Unicode characters by working with runes.
func CalculateLevenshteinDistance(a, b string) int {
	// Convert strings to rune slices to properly handle Unicode
	runesA := []rune(a)
	runesB := []rune(b)

	lenA := len(runesA)
	lenB := len(runesB)

	if lenA == 0 {
		return lenB
	}
	if lenB == 0 {
		return lenA
	}

	// Initialize the distance matrix
	// matrix[i][j] will be the Levenshtein distance between the first i characters of a
	// and the first j characters of b.
	matrix := make([][]int, lenA+1)
	for i := range matrix {
		matrix[i] = make([]int, lenB+1)
	}

	// Initialize the first row and column of the matrix
	for i := 0; i <= lenA; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= lenB; j++ {
		matrix[0][j] = j
	}

	// Fill the rest of the matrix
	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			cost := 0
			if runesA[i-1] != runesB[j-1] {
				cost = 1
			}

			// Minimum of (deletion, insertion, substitution)
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost

			matrix[i][j] = min3(deletion, insertion, substitution)
		}
	}

	return matrix[lenA][lenB]
}

// CalculateDamerauLevenshteinDistance computes the Damerau-Levenshtein distance between two strings.
// It represents the minimum number of single-character edits (insertions, deletions, substitutions, or transpositions)
// required to change one word into the other.
// This implementation properly handles Unicode characters by working with runes.
func CalculateDamerauLevenshteinDistance(a, b string) int {
	// Convert strings to rune slices to properly handle Unicode
	runesA := []rune(a)
	runesB := []rune(b)

	lenA := len(runesA)
	lenB := len(runesB)

	if lenA == 0 {
		return lenB
	}
	if lenB == 0 {
		return lenA
	}

	// Initialize the distance matrix
	// matrix[i][j] will be the Damerau-Levenshtein distance between the first i characters of a
	// and the first j characters of b.
	matrix := make([][]int, lenA+1)
	for i := range matrix {
		matrix[i] = make([]int, lenB+1)
	}

	// Initialize the first row and column of the matrix
	for i := 0; i <= lenA; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= lenB; j++ {
		matrix[0][j] = j
	}

	// Fill the rest of the matrix
	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			cost := 0
			if runesA[i-1] != runesB[j-1] {
				cost = 1
			}

			// Standard operations: deletion, insertion, substitution
			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost

			matrix[i][j] = min3(deletion, insertion, substitution)

			// Transposition operation (Damerau extension)
			// Check if we can do a transposition
			if i > 1 && j > 1 &&
				runesA[i-1] == runesB[j-2] &&
				runesA[i-2] == runesB[j-1] {
				transposition := matrix[i-2][j-2] + cost
				if transposition < matrix[i][j] {
					matrix[i][j] = transposition
				}
			}
		}
	}

	return matrix[lenA][lenB]
}

// CalculateDamerauLevenshteinDistanceWithLimit calculates Damerau-Levenshtein distance with early termination
// This includes transposition operations in addition to insertion, deletion, and substitution
// Returns maxDistance + 1 if the actual distance exceeds maxDistance (for performance)
func CalculateDamerauLevenshteinDistanceWithLimit(a, b string, maxDistance int) int {
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

		dist := CalculateDamerauLevenshteinDistanceWithLimit(term, indexedTerm, maxDistance)
		if dist > 0 && dist <= maxDistance { // dist > 0 ensures it's a different word
			typos = append(typos, indexedTerm)
		}
	}
	return typos
}

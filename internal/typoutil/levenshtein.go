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

			matrix[i][j] = min(deletion, insertion, substitution)
		}
	}

	return matrix[lenA][lenB]
}

// min is a helper function to find the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}

// GenerateTypos finds terms from a list of indexedTerms that are within a given maxDistance (Levenshtein) from the input term.
func GenerateTypos(term string, allIndexedTerms []string, maxDistance int) []string {
	typos := make([]string, 0) // Initialize as empty slice, not nil
	if maxDistance <= 0 || term == "" || len(allIndexedTerms) == 0 {
		return typos // Return empty slice instead of nil
	}

	for _, indexedTerm := range allIndexedTerms {
		// Avoid comparing the term to itself if it happens to be in allIndexedTerms already for typo generation
		if indexedTerm == term {
			continue
		}
		dist := CalculateLevenshteinDistance(term, indexedTerm)
		if dist > 0 && dist <= maxDistance { // dist > 0 ensures it's a different word
			typos = append(typos, indexedTerm)
		}
	}
	return typos
}

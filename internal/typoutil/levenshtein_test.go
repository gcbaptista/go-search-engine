package typoutil

import (
	"testing"
)

func TestCalculateLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"ab", "abc", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"café", "cafe", 1}, // Unicode test
	}

	for _, test := range tests {
		result := CalculateLevenshteinDistance(test.a, test.b)
		if result != test.expected {
			t.Errorf("CalculateLevenshteinDistance(%q, %q) = %d; expected %d", test.a, test.b, result, test.expected)
		}
	}
}

func TestCalculateDamerauLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
		note     string
	}{
		{"", "", 0, "empty strings"},
		{"", "abc", 3, "empty to non-empty"},
		{"abc", "", 3, "non-empty to empty"},
		{"abc", "abc", 0, "identical strings"},
		{"abc", "ab", 1, "deletion"},
		{"ab", "abc", 1, "insertion"},
		{"abc", "def", 3, "all substitutions"},
		{"kitten", "sitting", 3, "complex case"},
		{"saturday", "sunday", 3, "complex case"},
		{"café", "cafe", 1, "Unicode test"},

		// Transposition tests (key difference from standard Levenshtein)
		{"ab", "ba", 1, "simple transposition"},
		{"abc", "acb", 1, "transposition at end"},
		{"abc", "bac", 1, "transposition at start"},
		{"form", "from", 1, "common typo - transposition"},
		{"teh", "the", 1, "common typo - transposition"},
		{"recieve", "receive", 1, "common typo - ie/ei transposition"},
		{"calendar", "calender", 1, "ar/er transposition"},

		// Cases where transposition doesn't help
		{"abc", "xyz", 3, "no transposition benefit"},
		{"hello", "world", 4, "no transposition benefit"},

		// Multiple operations
		{"form", "forms", 1, "insertion after transposition candidate"},
		{"forms", "from", 2, "deletion + transposition"},
	}

	for _, test := range tests {
		result := CalculateDamerauLevenshteinDistance(test.a, test.b)
		if result != test.expected {
			t.Errorf("CalculateDamerauLevenshteinDistance(%q, %q) = %d; expected %d (%s)",
				test.a, test.b, result, test.expected, test.note)
		}
	}
}

func TestCalculateDamerauLevenshteinDistanceWithLimit(t *testing.T) {
	tests := []struct {
		a           string
		b           string
		maxDistance int
		expected    int
		note        string
	}{
		// Basic cases
		{"abc", "abc", 2, 0, "identical strings"},
		{"ab", "ba", 2, 1, "simple transposition"},
		{"form", "from", 2, 1, "common typo"},
		{"teh", "the", 2, 1, "common typo"},

		// Early termination cases
		{"abc", "xyz", 1, 2, "should return maxDistance+1 when exceeds limit"},
		{"hello", "world", 2, 3, "should return maxDistance+1 when exceeds limit"},

		// Length difference early termination
		{"a", "abcd", 2, 3, "length difference > maxDistance"},
		{"abcd", "a", 2, 3, "length difference > maxDistance"},

		// Edge cases
		{"", "", 1, 0, "empty strings"},
		{"", "a", 1, 1, "empty to single char"},
		{"a", "", 1, 1, "single char to empty"},
	}

	for _, test := range tests {
		result := CalculateDamerauLevenshteinDistanceWithLimit(test.a, test.b, test.maxDistance)
		if result != test.expected {
			t.Errorf("CalculateDamerauLevenshteinDistanceWithLimit(%q, %q, %d) = %d; expected %d (%s)",
				test.a, test.b, test.maxDistance, result, test.expected, test.note)
		}
	}
}

func TestGenerateTypos(t *testing.T) {
	indexedTerms := []string{"the", "form", "from", "farm", "firm", "fork", "receive", "recieve", "calendar", "calender"}

	tests := []struct {
		term        string
		maxDistance int
		expected    []string
		note        string
	}{
		{
			term:        "form",
			maxDistance: 1,
			expected:    []string{"from", "farm", "firm", "fork"}, // "from" now included due to transposition
			note:        "should include transposition matches",
		},
		{
			term:        "teh",
			maxDistance: 1,
			expected:    []string{"the"}, // transposition match
			note:        "common typo with transposition",
		},
		{
			term:        "recieve",
			maxDistance: 1,
			expected:    []string{"receive"}, // ie->ei transposition
			note:        "ie/ei transposition",
		},
		{
			term:        "calender",
			maxDistance: 1,
			expected:    []string{"calendar"}, // er->ar transposition
			note:        "er/ar transposition",
		},
		{
			term:        "xyz",
			maxDistance: 1,
			expected:    []string{},
			note:        "no matches within distance",
		},
	}

	for _, test := range tests {
		result := GenerateTypos(test.term, indexedTerms, test.maxDistance)

		// Convert to map for easier comparison
		resultMap := make(map[string]bool)
		for _, term := range result {
			resultMap[term] = true
		}

		expectedMap := make(map[string]bool)
		for _, term := range test.expected {
			expectedMap[term] = true
		}

		// Check if all expected terms are present
		for _, expectedTerm := range test.expected {
			if !resultMap[expectedTerm] {
				t.Errorf("GenerateTypos(%q, %d): missing expected term %q (%s)",
					test.term, test.maxDistance, expectedTerm, test.note)
			}
		}

		// Check if there are unexpected terms (this is more lenient, just log)
		for _, resultTerm := range result {
			if !expectedMap[resultTerm] {
				t.Logf("GenerateTypos(%q, %d): found additional term %q (may be valid)",
					test.term, test.maxDistance, resultTerm)
			}
		}
	}
}

// Benchmark comparison between standard Levenshtein and Damerau-Levenshtein
func BenchmarkLevenshteinVsDamerauLevenshtein(b *testing.B) {
	testCases := []struct {
		a, b string
	}{
		{"form", "from"},
		{"teh", "the"},
		{"recieve", "receive"},
		{"calendar", "calender"},
		{"kitten", "sitting"},
		{"saturday", "sunday"},
	}

	b.Run("Standard Levenshtein", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, tc := range testCases {
				CalculateLevenshteinDistance(tc.a, tc.b)
			}
		}
	})

	b.Run("Damerau-Levenshtein", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, tc := range testCases {
				CalculateDamerauLevenshteinDistance(tc.a, tc.b)
			}
		}
	})

	b.Run("Damerau-Levenshtein WithLimit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, tc := range testCases {
				CalculateDamerauLevenshteinDistanceWithLimit(tc.a, tc.b, 2)
			}
		}
	})
}

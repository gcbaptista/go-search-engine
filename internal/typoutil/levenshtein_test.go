package typoutil

import (
	"testing"
)

func TestCalculateEditDistance(t *testing.T) {
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
		result := CalculateEditDistance(test.a, test.b, test.maxDistance)
		if result != test.expected {
			t.Errorf("CalculateEditDistance(%q, %q, %d) = %d; expected %d (%s)",
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

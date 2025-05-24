package typoutil

import (
	"reflect"
	"sort"
	"testing"
)

func TestCalculateLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"both empty", "", "", 0},
		{"a empty", "", "hello", 5},
		{"b empty", "hello", "", 5},
		{"identical", "hello", "hello", 0},
		{"simple substitution", "kitten", "sitten", 1},
		{"simple insertion", "apple", "applye", 1},
		{"simple deletion", "banana", "banna", 1},
		{"multiple edits", "saturday", "sunday", 3},
		{"order matters", "apple", "applye", 1},
		{"order matters reverse", "applye", "apple", 1},
		{"longer strings", "algorithm", "altruistic", 6},
		{"unicode chars (same len)", "cliché", "cliche", 1}, // é -> e is 1 substitution
		{"unicode chars (diff len)", "résumé", "resume", 2}, // é -> e twice is 2 substitutions
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateLevenshteinDistance(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CalculateLevenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGenerateTypos(t *testing.T) {
	allIndexedTerms := []string{"apple", "apply", "apricot", "banana", "bandana", "orange", "search", "serch", "seech"}

	tests := []struct {
		name         string
		term         string
		indexedTerms []string
		maxDistance  int
		want         []string
	}{
		{"no typos, exact match in list", "apple", allIndexedTerms, 1, []string{"apply"}}, // "apple" itself is skipped
		{"single typo found", "serch", allIndexedTerms, 1, []string{"search", "seech"}},
		{"multiple typos found", "aple", allIndexedTerms, 1, []string{"apple"}}, // "apply" has distance 2, so excluded
		{"no typos, distance too small", "apricot", allIndexedTerms, 0, []string{}},
		{"no typos, term not similar to any", "kiwi", allIndexedTerms, 2, []string{}},
		{"typos with distance 2", "serc", allIndexedTerms, 2, []string{"search", "serch", "seech"}},
		{"empty indexed terms", "apple", []string{}, 1, []string{}},
		{"empty term", "", allIndexedTerms, 1, []string{}}, // Levenshtein of "" to "word" is len(word)
		{"maxDistance 0", "apple", allIndexedTerms, 0, []string{}},
		{"term is in list, check others", "bandana", allIndexedTerms, 1, []string{"banana"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateTypos(tt.term, tt.indexedTerms, tt.maxDistance)
			// Sort both slices for consistent comparison as order doesn't matter for typos
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateTypos(%q, ..., %d) = %v, want %v", tt.term, tt.maxDistance, got, tt.want)
			}
		})
	}
}

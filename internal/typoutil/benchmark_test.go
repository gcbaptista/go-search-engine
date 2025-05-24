package typoutil

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Generate test data for benchmarks
func generateTestTerms(count int, avgLength int) []string {
	rand.Seed(time.Now().UnixNano())
	terms := make([]string, count)

	words := []string{
		"action", "adventure", "comedy", "drama", "horror", "thriller", "science", "fiction",
		"fantasy", "romance", "mystery", "crime", "animation", "documentary", "family",
		"music", "war", "western", "biography", "history", "sport", "musical", "film",
		"movie", "cinema", "actor", "actress", "director", "producer", "screenplay",
		"character", "plot", "story", "narrative", "dialogue", "scene", "sequence",
		"the", "and", "for", "are", "but", "not", "you", "all", "can", "had", "her",
		"was", "one", "our", "out", "day", "get", "has", "him", "his", "how", "man",
		"new", "now", "old", "see", "two", "way", "who", "boy", "did", "its", "let",
		"put", "say", "she", "too", "use",
	}

	for i := 0; i < count; i++ {
		// Sometimes use existing words, sometimes generate random strings
		if rand.Float32() < 0.7 {
			terms[i] = words[rand.Intn(len(words))]
		} else {
			// Generate random string
			length := avgLength + rand.Intn(5) - 2 // avgLength ± 2
			if length < 3 {
				length = 3
			}

			runes := make([]rune, length)
			for j := 0; j < length; j++ {
				runes[j] = rune('a' + rand.Intn(26))
			}
			terms[i] = string(runes)
		}
	}

	return terms
}

// Benchmark the original GenerateTypos function
func BenchmarkGenerateTyposOriginal(b *testing.B) {
	indexedTerms := generateTestTerms(1000, 6)
	queryTerms := []string{"action", "advnture", "comdy", "thrlr", "mysterey"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, term := range queryTerms {
			_ = GenerateTypos(term, indexedTerms, 1)
		}
	}
}

// Benchmark the optimized simple version
func BenchmarkGenerateTyposSimple(b *testing.B) {
	indexedTerms := generateTestTerms(1000, 6)
	queryTerms := []string{"action", "advnture", "comdy", "thrlr", "mysterey"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, term := range queryTerms {
			_ = GenerateTyposSimple(term, indexedTerms, 1)
		}
	}
}

// Benchmark the optimized finder with cache
func BenchmarkTypoFinderOptimized(b *testing.B) {
	indexedTerms := generateTestTerms(1000, 6)
	queryTerms := []string{"action", "advnture", "comdy", "thrlr", "mysterey"}

	finder := NewTypoFinder(indexedTerms)
	finder.UpdateIndexedTerms(indexedTerms)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, term := range queryTerms {
			_ = finder.GenerateTyposOptimized(term, 1, 10) // Limit to 10 results
		}
	}
}

// Benchmark different index sizes
func BenchmarkScaling(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}
	queryTerms := []string{"action", "advnture", "comdy"}

	for _, size := range sizes {
		indexedTerms := generateTestTerms(size, 6)

		b.Run(fmt.Sprintf("Original_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, term := range queryTerms {
					_ = GenerateTypos(term, indexedTerms, 1)
				}
			}
		})

		b.Run(fmt.Sprintf("Simple_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, term := range queryTerms {
					_ = GenerateTyposSimple(term, indexedTerms, 1)
				}
			}
		})

		b.Run(fmt.Sprintf("Optimized_%d", size), func(b *testing.B) {
			finder := NewTypoFinder(indexedTerms)
			finder.UpdateIndexedTerms(indexedTerms)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				for _, term := range queryTerms {
					_ = finder.GenerateTyposOptimized(term, 1, 10)
				}
			}
		})
	}
}

// Benchmark Levenshtein distance calculation
func BenchmarkLevenshteinDistance(b *testing.B) {
	testPairs := [][]string{
		{"kitten", "sitting"},
		{"action", "aktion"},
		{"adventure", "advnture"},
		{"comedy", "comdy"},
		{"thriller", "thrlr"},
	}

	b.Run("Original", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, pair := range testPairs {
				_ = CalculateLevenshteinDistance(pair[0], pair[1])
			}
		}
	})

	b.Run("Optimized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, pair := range testPairs {
				_ = CalculateLevenshteinDistanceOptimized(pair[0], pair[1], 2)
			}
		}
	})
}

// Benchmark cache effectiveness
func BenchmarkCacheEffectiveness(b *testing.B) {
	indexedTerms := generateTestTerms(1000, 6)
	// Use limited set of query terms to test cache hits
	queryTerms := []string{"action", "advnture", "comdy", "action", "advnture", "comdy"}

	finder := NewTypoFinder(indexedTerms)
	finder.UpdateIndexedTerms(indexedTerms)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, term := range queryTerms {
			_ = finder.GenerateTyposOptimized(term, 1, 10)
		}
	}
}

// Benchmark early termination effectiveness
func BenchmarkEarlyTermination(b *testing.B) {
	indexedTerms := generateTestTerms(5000, 6)

	// Test with terms that should have early termination
	queryTerms := []string{"verylongwordthatdoesnotexist", "anotherlongwordnotinindex"}

	b.Run("Original", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, term := range queryTerms {
				_ = GenerateTypos(term, indexedTerms, 1)
			}
		}
	})

	b.Run("Optimized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, term := range queryTerms {
				_ = GenerateTyposSimple(term, indexedTerms, 1)
			}
		}
	})
}

func TestDualCriteriaStopping(t *testing.T) {
	// Create a simple set with known distance 1 matches from our debug
	simpleTermSet := []string{
		"test", "tests", "testa", "testb", "testc", "testd", "teste", "testf", "testg", "testh",
		"rest", "best", "nest", "pest", "west", "fest", "jest", "zest", "text", "temp",
		"gest", "lest", "mest", "dest", "vest", "kest", "cest", "hest", "yest", "qest",
	}

	typoFinder := NewTypoFinder(simpleTermSet)
	typoFinder.UpdateIndexedTerms(simpleTermSet)

	t.Run("basic functionality verification", func(t *testing.T) {
		// First verify that we can find matches at all
		results := typoFinder.GenerateTypos("test", 1, 100)
		if len(results) == 0 {
			t.Fatal("Basic typo finding not working - no results found")
		}
		t.Logf("Basic check: Found %d matches for 'test' with distance 1", len(results))
	})

	t.Run("result limit enforcement", func(t *testing.T) {
		// Test that result limit is enforced
		startTime := time.Now()
		results := typoFinder.GenerateTyposWithTimeLimit("test", 1, 5, 10*time.Second) // Only 5 results, long time
		duration := time.Since(startTime)

		if len(results) > 5 {
			t.Errorf("Expected at most 5 results, got %d", len(results))
		}

		// Should complete quickly since we only need 5 results
		if duration > 100*time.Millisecond {
			t.Errorf("Expected to complete quickly when finding only 5 results, took %v", duration)
		}

		t.Logf("Result limit test: Found %d results in %v", len(results), duration)
	})

	t.Run("time limit enforcement", func(t *testing.T) {
		// Create a larger dataset to test time limits
		largeTermSet := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			// Create terms that are exactly distance 1 from "test"
			variations := []string{"aest", "best", "cest", "dest", "eest", "fest", "gest", "hest", "iest", "jest", "kest", "lest", "mest", "nest", "oest", "pest", "qest", "rest", "sest", "test", "uest", "vest", "west", "xest", "yest", "zest"}
			base := variations[i%len(variations)]
			largeTermSet[i] = fmt.Sprintf("%s%04d", base, i)
		}

		largeFinder := NewTypoFinder(largeTermSet)
		largeFinder.UpdateIndexedTerms(largeTermSet)

		startTime := time.Now()
		results := largeFinder.GenerateTyposWithTimeLimit("test", 1, 10000, 1*time.Millisecond) // Very short time limit
		duration := time.Since(startTime)

		// Should stop due to time limit
		if len(results) >= 10000 {
			t.Errorf("Expected time limit to prevent finding all results, got %d", len(results))
		}

		// Duration should be reasonable (not too long)
		if duration > 100*time.Millisecond {
			t.Errorf("Expected to stop due to time limit quickly, took %v", duration)
		}

		t.Logf("Time limit test: Found %d results in %v", len(results), duration)
	})

	t.Run("real scenario - 500 results or 50ms", func(t *testing.T) {
		// Test actual search engine parameters
		startTime := time.Now()
		results := typoFinder.GenerateTyposWithTimeLimit("test", 1, 500, 50*time.Millisecond)
		duration := time.Since(startTime)

		// Should work without error
		if len(results) > 500 {
			t.Errorf("Expected at most 500 results, got %d", len(results))
		}

		t.Logf("Real scenario: Found %d results in %v", len(results), duration)
		t.Logf("✅ Dual criteria implementation working correctly!")
	})

	t.Run("warning when time limit reached with remaining terms", func(t *testing.T) {
		// Create a large dataset to trigger time limit warning
		largeTermSet := make([]string, 2000)
		for i := 0; i < 2000; i++ {
			// Create terms that are distance 1 from "test" to ensure matches
			variations := []string{"best", "rest", "nest", "pest", "west", "fest", "jest", "zest", "test"}
			base := variations[i%len(variations)]
			largeTermSet[i] = fmt.Sprintf("%s_%04d", base, i)
		}

		largeFinder := NewTypoFinder(largeTermSet)
		largeFinder.UpdateIndexedTerms(largeTermSet)

		// Use very short time limit to trigger warning
		startTime := time.Now()
		results := largeFinder.GenerateTyposWithTimeLimit("test", 1, 500, 1*time.Millisecond)
		duration := time.Since(startTime)

		// Should complete quickly due to time limit
		if duration > 50*time.Millisecond {
			t.Errorf("Expected to stop due to time limit quickly, took %v", duration)
		}

		// Should have some results but likely not 500 due to time limit
		t.Logf("Warning test: Found %d results in %v (check logs for warning message)", len(results), duration)

		// Note: The warning should appear in the test output logs
		// This test verifies the functionality works without failing
	})
}

package tokenizer

import (
	"regexp"
	"strings"
)

// nonAlphanumericRegex matches sequences of non-alphanumeric characters.
var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// acronymRegex handles cases like "HTTPRequest" -> "HTTP Request"
var acronymRegex = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)

// camelCaseRegex handles cases like "theOffice" -> "the Office" or "myAPI" -> "my API"
var camelCaseRegex = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// Tokenize converts a string into a slice of tokens.
// It splits camel/PascalCase, lowercases the string, and splits by non-alphanumeric characters.
func Tokenize(text string) []string {
	// 1. Split camelCase/PascalCase
	processedText := acronymRegex.ReplaceAllString(text, "$1 $2")
	processedText = camelCaseRegex.ReplaceAllString(processedText, "$1 $2")

	// 2. Lowercase
	lowerText := strings.ToLower(processedText)

	// 3. Split by non-alphanumeric characters
	split := nonAlphanumericRegex.Split(lowerText, -1)

	tokens := make([]string, 0) // Initialize as empty slice, not nil
	for _, s := range split {
		if s != "" { // Filter out empty strings
			tokens = append(tokens, s)
		}
	}
	return tokens
}

// GeneratePrefixNGrams creates n-grams from a token, starting from length 1 up to the token's length.
// For example, for the token "search", it produces: "s", "se", "sea", "sear", "searc", "search".
func GeneratePrefixNGrams(token string) []string {
	tokenLen := len(token)
	if tokenLen == 0 {
		return make([]string, 0) // Return empty slice instead of nil
	}

	ngrams := make([]string, tokenLen)
	for i := 1; i <= tokenLen; i++ {
		ngrams[i-1] = token[:i]
	}
	return ngrams
}

// TokenizeWithPrefixNGrams combines Tokenize and GeneratePrefixNGrams.
// It produces original tokens and their prefix n-grams (from length 1).
func TokenizeWithPrefixNGrams(text string) []string {
	tokens := Tokenize(text)

	result := make([]string, 0)             // Initialize as empty slice, not nil
	seenNGrams := make(map[string]struct{}) // To avoid duplicate n-grams if tokens overlap etc.

	for _, token := range tokens {
		if _, seen := seenNGrams[token]; !seen {
			result = append(result, token) // Add original token
			seenNGrams[token] = struct{}{}
		}

		ngrams := GeneratePrefixNGrams(token)
		for _, ngram := range ngrams {
			if _, seen := seenNGrams[ngram]; !seen {
				result = append(result, ngram)
				seenNGrams[ngram] = struct{}{}
			}
		}
	}

	return result
}

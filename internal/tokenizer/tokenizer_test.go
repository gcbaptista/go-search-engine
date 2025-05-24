package tokenizer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", []string{}},
		{"simple lowercase", "hello world", []string{"hello", "world"}},
		{"with punctuation", "hello, world!", []string{"hello", "world"}},
		{"with numbers", "item123 test", []string{"item123", "test"}},
		{"leading/trailing spaces", "  hello world  ", []string{"hello", "world"}},
		{"multiple spaces between words", "hello   world", []string{"hello", "world"}},
		{"camelCase", "theOffice", []string{"the", "office"}},
		{"PascalCase", "TheOffice", []string{"the", "office"}},
		{"mixedCase", "myAPIService", []string{"my", "api", "service"}},
		{"acronym then camelCase", "HTTPRequestManager", []string{"http", "request", "manager"}},
		{"acronym at end", "performHTTPRequest", []string{"perform", "http", "request"}},
		{"string with hyphen", "state-of-the-art", []string{"state", "of", "the", "art"}},
		{"string with underscore", "my_variable_name", []string{"my", "variable", "name"}},
		{"all caps word", "HELLO WORLD", []string{"hello", "world"}},
		{"mixed with numbers and symbols", "API_v1.0-beta!", []string{"api", "v1", "0", "beta"}},
		{"starts with digit then uppercase", "1Password", []string{"1", "password"}},
		{"only symbols", "!@#$%^", []string{}},
		{"only numbers", "12345 67890", []string{"12345", "67890"}},
		{"complex acronym", "BIGAcronymThenCamel", []string{"big", "acronym", "then", "camel"}},
		{"another camel case", "anotherCase", []string{"another", "case"}},
		{"special chars in middle", "word1!@#word2", []string{"word1", "word2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokenize(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGeneratePrefixNGrams(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  []string
	}{
		{"empty token", "", []string{}},
		{"single character", "a", []string{"a"}},
		{"short token", "cat", []string{"c", "ca", "cat"}},
		{"longer token", "search", []string{"s", "se", "sea", "sear", "searc", "search"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePrefixNGrams(tt.token)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GeneratePrefixNGrams(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestTokenizeWithPrefixNGrams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", []string{}},
		{"simple word", "cat", []string{"cat", "c", "ca"}},
		{"two words", "cat dog", []string{"cat", "c", "ca", "dog", "d", "do"}},
		{"camelCase word", "theOffice", []string{"the", "t", "th", "office", "o", "of", "off", "offi", "offic"}},
		{"word with punctuation", "api-v1", []string{"api", "a", "ap", "v1", "v"}},
		{"duplicate tokens", "test test", []string{"test", "t", "te", "tes"}},
		{"overlapping ngrams", "tester testing", []string{
			"tester", "t", "te", "tes", "test", "teste",
			"testing", "testi", "testin",
		}},
		{"empty after tokenize", "!@#$", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenizeWithPrefixNGrams(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TokenizeWithPrefixNGrams(%q): got %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
		})
	}
}

func TestTokenize_EdgeCases(t *testing.T) {
	input1 := "1Password"
	want1 := []string{"1", "password"}
	got1 := Tokenize(input1)
	if !reflect.DeepEqual(got1, want1) {
		t.Errorf("Tokenize(%q) = %v, want %v", input1, got1, want1)
	}

	input2 := "myAPI1Test"
	want2 := []string{"my", "api1", "test"}
	got2 := Tokenize(input2)
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("Tokenize(%q) = %v, want %v", input2, got2, want2)
	}
}

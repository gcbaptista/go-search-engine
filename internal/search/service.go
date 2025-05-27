package search

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/internal/tokenizer"
	"github.com/gcbaptista/go-search-engine/internal/typoutil"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
	"github.com/gcbaptista/go-search-engine/store"
	"github.com/google/uuid"
)

// Service implements the search logic for a single index.
// It will fulfill the services.Searcher interface.
type Service struct {
	invertedIndex *index.InvertedIndex
	documentStore *store.DocumentStore
	settings      *config.IndexSettings
	typoFinder    *typoutil.TypoFinder // Typo finder with caching
}

// NewService creates a new search Service.
func NewService(invIndex *index.InvertedIndex, docStore *store.DocumentStore, settings *config.IndexSettings) (*Service, error) {
	if invIndex == nil {
		return nil, fmt.Errorf("inverted index cannot be nil")
	}
	if docStore == nil {
		return nil, fmt.Errorf("document store cannot be nil")
	}
	if settings == nil {
		return nil, fmt.Errorf("settings cannot be nil")
	}

	// Create indexed terms slice for typo finder
	indexedTerms := make([]string, 0, len(invIndex.Index))
	for term := range invIndex.Index {
		indexedTerms = append(indexedTerms, term)
	}

	// Initialize typo finder
	typoFinder := typoutil.NewTypoFinder(indexedTerms)
	typoFinder.UpdateIndexedTerms(indexedTerms)

	return &Service{
		invertedIndex: invIndex,
		documentStore: docStore,
		settings:      settings,
		typoFinder:    typoFinder,
	}, nil
}

// UpdateTypoFinder updates the typo finder's indexed terms.
// This should be called after documents are added to keep the typo finder in sync.
func (s *Service) UpdateTypoFinder() {
	// Get current indexed terms
	indexedTerms := make([]string, 0, len(s.invertedIndex.Index))
	for term := range s.invertedIndex.Index {
		indexedTerms = append(indexedTerms, term)
	}

	// Update the typo finder
	s.typoFinder.UpdateIndexedTerms(indexedTerms)
}

const defaultPageSize = 10

// Search performs a search operation based on the query.
func (s *Service) Search(query services.SearchQuery) (services.SearchResult, error) {
	startTime := time.Now()

	// Determine effective searchable fields based on query and index settings
	var effectiveSearchableFields []string
	var isFieldAllowed func(string) bool

	if len(query.RestrictSearchableFields) > 0 {
		// RestrictSearchableFields provided - validate and AND with configured searchable fields
		configuredFields := make(map[string]bool)
		for _, field := range s.settings.SearchableFields {
			configuredFields[field] = true
		}

		// Validate that restricted fields are a subset of configured searchable fields
		for _, restrictedField := range query.RestrictSearchableFields {
			if !configuredFields[restrictedField] {
				return services.SearchResult{}, fmt.Errorf("restricted searchable field '%s' is not configured as a searchable field in index settings", restrictedField)
			}
		}

		// Use the intersection (AND) of RestrictSearchableFields with configured searchable fields
		effectiveSearchableFields = query.RestrictSearchableFields

		// Create field restriction checker
		allowedFields := make(map[string]bool)
		for _, field := range effectiveSearchableFields {
			allowedFields[field] = true
		}

		isFieldAllowed = func(fieldName string) bool {
			return allowedFields[fieldName]
		}
	} else {
		// RestrictSearchableFields not provided - use all configured searchable fields
		effectiveSearchableFields = s.settings.SearchableFields

		// Create field restriction checker
		allowedFields := make(map[string]bool)
		for _, field := range effectiveSearchableFields {
			allowedFields[field] = true
		}

		isFieldAllowed = func(fieldName string) bool {
			return allowedFields[fieldName]
		}
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	originalQueryTokens := tokenizer.Tokenize(query.QueryString)
	if len(originalQueryTokens) == 0 {
		queryUUID := uuid.New().String()
		return services.SearchResult{Hits: []services.HitResult{}, Total: 0, Page: page, PageSize: pageSize, Took: time.Since(startTime).Milliseconds(), QueryId: queryUUID}, nil
	}

	s.invertedIndex.Mu.RLock()
	s.documentStore.Mu.RLock()
	defer s.invertedIndex.Mu.RUnlock()
	defer s.documentStore.Mu.RUnlock()

	// Per query token, store map of DocID to list of posting entries (can match multiple fields)
	docMatchesByQueryToken := make(map[string]map[uint32][]index.PostingEntry)
	// For typo suggestions, we also need to know which original query token a typo belongs to.
	// Map: originalQueryToken -> docID -> []PostingEntry
	docMatchesByOriginalQueryTokenForTypos := make(map[string]map[uint32][]index.PostingEntry)
	// Track which typo term was actually matched for each original query token and docID
	// Map: originalQueryToken -> docID -> []actualTypoTerm
	typoTermsMatchedByQueryToken := make(map[string]map[uint32][]string)
	// Track the best typo distance for each query token and document to avoid redundant worse matches
	// Map: originalQueryToken -> docID -> bestTypoDistance
	bestTypoDistanceByQueryToken := make(map[string]map[uint32]int)

	// First pass: collect exact matches for all query tokens
	for _, queryToken := range originalQueryTokens {
		docMatchesByQueryToken[queryToken] = make(map[uint32][]index.PostingEntry)
		docMatchesByOriginalQueryTokenForTypos[queryToken] = make(map[uint32][]index.PostingEntry)
		typoTermsMatchedByQueryToken[queryToken] = make(map[uint32][]string)
		bestTypoDistanceByQueryToken[queryToken] = make(map[uint32]int)

		// 1. Exact matches for the queryToken
		if postingList, found := s.invertedIndex.Index[queryToken]; found {
			for _, entry := range postingList {
				if isFieldAllowed(entry.FieldName) {
					docMatchesByQueryToken[queryToken][entry.DocID] = append(docMatchesByQueryToken[queryToken][entry.DocID], entry)
				}
			}
		}
	}

	// Second pass: apply typo tolerance (skip if document already has exact match for the specific token)
	for _, queryToken := range originalQueryTokens {

		// 2. Typo matches for the queryToken
		// Check if this query token is in the non-typo tolerant words list
		isNonTypoTolerant := false
		for _, nonTypoWord := range s.settings.NonTypoTolerantWords {
			if strings.EqualFold(queryToken, nonTypoWord) {
				isNonTypoTolerant = true
				break
			}
		}

		// Skip typo matching if this word is in the non-typo tolerant list
		if !isNonTypoTolerant {
			// Use dual criteria: stop when either 500 tokens found OR 50ms elapsed
			maxTypoResults := 500
			timeLimit := 50 * time.Millisecond

			// Use query-level minWordSize settings if provided, otherwise fall back to index settings
			minWordSizeFor1Typo := s.settings.MinWordSizeFor1Typo
			if query.MinWordSizeFor1Typo != nil {
				minWordSizeFor1Typo = *query.MinWordSizeFor1Typo
			}

			minWordSizeFor2Typos := s.settings.MinWordSizeFor2Typos
			if query.MinWordSizeFor2Typos != nil {
				minWordSizeFor2Typos = *query.MinWordSizeFor2Typos
			}

			if minWordSizeFor1Typo > 0 && len(queryToken) >= minWordSizeFor1Typo {
				typos1 := s.typoFinder.GenerateTyposWithTimeLimit(queryToken, 1, maxTypoResults, timeLimit)
				for _, typoTerm := range typos1 {
					// Skip if the typo term is the same as the original query token
					if typoTerm == queryToken {
						continue
					}

					// Check if the typo term itself is in the non-typo tolerant words list
					// or if it's a prefix that could match non-typo tolerant words
					isTypoTermNonTypoTolerant := false
					for _, nonTypoWord := range s.settings.NonTypoTolerantWords {
						if strings.EqualFold(typoTerm, nonTypoWord) {
							isTypoTermNonTypoTolerant = true
							break
						}
						// Also check if the typo term is a prefix of a non-typo tolerant word
						// This prevents partial matches like "stal" matching documents with "stalin"
						if len(typoTerm) >= 3 && strings.HasPrefix(strings.ToLower(nonTypoWord), strings.ToLower(typoTerm)) {
							isTypoTermNonTypoTolerant = true
							break
						}
					}

					// Skip this typo if it would match a non-typo tolerant word
					if isTypoTermNonTypoTolerant {
						continue
					}

					if postingList, found := s.invertedIndex.Index[typoTerm]; found {
						for _, entry := range postingList {
							if isFieldAllowed(entry.FieldName) {
								// Skip typo matching for documents that already have exact matches for this specific query token
								if _, hasExactMatch := docMatchesByQueryToken[queryToken][entry.DocID]; hasExactMatch {
									continue
								}

								// Check if we already have a better (lower distance) typo match for this query token in this document
								currentBestDistance, hasPreviousTypo := bestTypoDistanceByQueryToken[queryToken][entry.DocID]
								if hasPreviousTypo && currentBestDistance <= 1 {
									continue // Skip this 1-typo match since we already have an equal or better match
								}

								typoEntry := entry
								typoEntry.Score *= 0.8 // Penalize typo scores slightly

								// If this is a better match, replace previous typo matches for this document and query token
								if !hasPreviousTypo || 1 < currentBestDistance {
									docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID] = []index.PostingEntry{typoEntry}
									typoTermsMatchedByQueryToken[queryToken][entry.DocID] = []string{typoTerm}
									bestTypoDistanceByQueryToken[queryToken][entry.DocID] = 1
								} else if 1 == currentBestDistance {
									// Same distance, add to existing matches
									docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID] = append(docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID], typoEntry)
									typoTermsMatchedByQueryToken[queryToken][entry.DocID] = append(typoTermsMatchedByQueryToken[queryToken][entry.DocID], typoTerm)
								}
							}
						}
					}
				}
			}

			if minWordSizeFor2Typos > 0 && len(queryToken) >= minWordSizeFor2Typos {
				typos2 := s.typoFinder.GenerateTyposWithTimeLimit(queryToken, 2, maxTypoResults, timeLimit)
				for _, typoTerm := range typos2 {
					// Skip if the typo term is the same as the original query token
					if typoTerm == queryToken {
						continue
					}

					// Check if the typo term itself is in the non-typo tolerant words list
					// or if it's a prefix that could match non-typo tolerant words
					isTypoTermNonTypoTolerant := false
					for _, nonTypoWord := range s.settings.NonTypoTolerantWords {
						if strings.EqualFold(typoTerm, nonTypoWord) {
							isTypoTermNonTypoTolerant = true
							break
						}
						// Also check if the typo term is a prefix of a non-typo tolerant word
						// This prevents partial matches like "stal" matching documents with "stalin"
						if len(typoTerm) >= 3 && strings.HasPrefix(strings.ToLower(nonTypoWord), strings.ToLower(typoTerm)) {
							isTypoTermNonTypoTolerant = true
							break
						}
					}

					// Skip this typo if it would match a non-typo tolerant word
					if isTypoTermNonTypoTolerant {
						continue
					}

					if postingList, found := s.invertedIndex.Index[typoTerm]; found {
						for _, entry := range postingList {
							if isFieldAllowed(entry.FieldName) {
								// Skip typo matching for documents that already have exact matches for this specific query token
								if _, hasExactMatch := docMatchesByQueryToken[queryToken][entry.DocID]; hasExactMatch {
									continue
								}

								// Check if we already have a better (lower distance) typo match for this query token in this document
								currentBestDistance, hasPreviousTypo := bestTypoDistanceByQueryToken[queryToken][entry.DocID]
								if hasPreviousTypo && currentBestDistance <= 2 {
									continue // Skip this 2-typo match since we already have an equal or better match
								}

								typoEntry := entry
								typoEntry.Score *= 0.6 // Penalize 2-typo matches more than 1-typo

								// If this is a better match, replace previous typo matches for this document and query token
								if !hasPreviousTypo || 2 < currentBestDistance {
									docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID] = []index.PostingEntry{typoEntry}
									typoTermsMatchedByQueryToken[queryToken][entry.DocID] = []string{typoTerm}
									bestTypoDistanceByQueryToken[queryToken][entry.DocID] = 2
								} else if 2 == currentBestDistance {
									// Same distance, add to existing matches
									docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID] = append(docMatchesByOriginalQueryTokenForTypos[queryToken][entry.DocID], typoEntry)
									typoTermsMatchedByQueryToken[queryToken][entry.DocID] = append(typoTermsMatchedByQueryToken[queryToken][entry.DocID], typoTerm)
								}
							}
						}
					}
				}
			}
		}
	}

	// Find intersection of DocIDs: documents that match ALL originalQueryTokens (either exactly or via typo)
	intersectedDocIDs := make(map[uint32]bool)
	if len(originalQueryTokens) > 0 {
		firstToken := originalQueryTokens[0]
		// Include docs that matched the first token either exactly or via typo
		for docID := range docMatchesByQueryToken[firstToken] {
			intersectedDocIDs[docID] = true
		}
		for docID := range docMatchesByOriginalQueryTokenForTypos[firstToken] {
			intersectedDocIDs[docID] = true // also consider docs that matched via typo
		}

		for i := 1; i < len(originalQueryTokens); i++ {
			token := originalQueryTokens[i]
			currentDocIDsForToken := make(map[uint32]bool)
			for docID := range docMatchesByQueryToken[token] {
				currentDocIDsForToken[docID] = true
			}
			for docID := range docMatchesByOriginalQueryTokenForTypos[token] {
				currentDocIDsForToken[docID] = true
			}

			newIntersectedDocIDs := make(map[uint32]bool)
			for docID := range intersectedDocIDs {
				if currentDocIDsForToken[docID] {
					newIntersectedDocIDs[docID] = true
				}
			}
			intersectedDocIDs = newIntersectedDocIDs
			if len(intersectedDocIDs) == 0 {
				break // No common documents left
			}
		}
	}

	// Build final hits from intersectedDocIDs
	// candidateHit type is now defined in types.go
	finalCandidateHits := make(map[uint32]*candidateHit) // docID -> candidateHit

	for docID := range intersectedDocIDs {
		doc, found := s.documentStore.Docs[docID]
		if !found {
			log.Printf("Warning: Document with internal ID %d in intersection but not in document store.\n", docID)
			continue
		}

		// Apply filter expression if any
		var filterScore float64
		if query.Filters != nil {
			matches, score := s.evaluateFilters(doc, *query.Filters)
			if !matches {
				continue
			}
			filterScore = score
		}

		currentHit := &candidateHit{
			doc:                      doc,
			score:                    0,
			filterScore:              filterScore,
			matchedQueryTermsByField: make(map[string]map[string]struct{}),
		}

		// Aggregate scores and matched fields for this docID from all query tokens
		for _, queryToken := range originalQueryTokens {
			// Track the best score for this query token for this document
			bestScoreForToken := 0.0

			// Exact matches
			if entries, ok := docMatchesByQueryToken[queryToken][docID]; ok {
				for _, entry := range entries {
					if isFieldAllowed(entry.FieldName) {
						if entry.Score > bestScoreForToken {
							bestScoreForToken = entry.Score
						}
						if _, fieldMapExists := currentHit.matchedQueryTermsByField[entry.FieldName]; !fieldMapExists {
							currentHit.matchedQueryTermsByField[entry.FieldName] = make(map[string]struct{})
						}
						currentHit.matchedQueryTermsByField[entry.FieldName][queryToken] = struct{}{}
					}
				}
			}

			// Typo matches (attributed to the original query token)
			if entries, ok := docMatchesByOriginalQueryTokenForTypos[queryToken][docID]; ok {
				typoTerms := typoTermsMatchedByQueryToken[queryToken][docID]
				for i, entry := range entries {
					if isFieldAllowed(entry.FieldName) {
						// Only use typo score if it's better than exact match score
						// (this should rarely happen, but protects against edge cases)
						if entry.Score > bestScoreForToken {
							bestScoreForToken = entry.Score
						}
						if _, fieldMapExists := currentHit.matchedQueryTermsByField[entry.FieldName]; !fieldMapExists {
							currentHit.matchedQueryTermsByField[entry.FieldName] = make(map[string]struct{})
						}
						// Mark typo matches for display using the actual matched typo term
						var matchDisplay string
						if i < len(typoTerms) {
							matchDisplay = typoTerms[i] + "(typo)"
						} else {
							matchDisplay = queryToken + "(typo)" // fallback
						}
						currentHit.matchedQueryTermsByField[entry.FieldName][matchDisplay] = struct{}{}
					}
				}
			}

			// Add the best score for this query token to the total
			currentHit.score += bestScoreForToken
		}
		finalCandidateHits[docID] = currentHit
	}

	// Convert finalCandidateHits map to a slice for sorting
	finalSelectHits := make([]services.HitResult, 0, len(finalCandidateHits))
	for _, ch := range finalCandidateHits {
		matchedTermsResult := make(map[string][]string)
		numTyposForHit := 0
		numberExactWordsForHit := 0
		uniqueMatchedOriginalQueryTokensTypos := make(map[string]struct{})
		uniqueOriginalQueryTokensExact := make(map[string]struct{})

		// Get full words from all searchable fields of the current document for exactness checking
		docFullWordsByField := make(map[string][]string)
		for _, searchableFieldName := range effectiveSearchableFields {
			if fieldValue, ok := ch.doc[searchableFieldName]; ok {
				var textContent string
				switch v := fieldValue.(type) {
				case string:
					textContent = v
				case []interface{}:
					var parts []string
					for _, item := range v {
						if strItem, isStr := item.(string); isStr {
							parts = append(parts, strItem)
						}
					}
					textContent = strings.Join(parts, " ")
				case []string:
					textContent = strings.Join(v, " ")
				}
				if textContent != "" {
					docFullWordsByField[searchableFieldName] = tokenizer.Tokenize(textContent)
				}
			}
		}

		for fieldName, tokensMap := range ch.matchedQueryTermsByField {
			for tokenFromMap := range tokensMap {
				matchedTermsResult[fieldName] = append(matchedTermsResult[fieldName], tokenFromMap)

				originalQueryTermForMatch := strings.Split(tokenFromMap, "(typo)")[0]

				if strings.Contains(tokenFromMap, "(typo)") {
					if _, alreadyCounted := uniqueMatchedOriginalQueryTokensTypos[originalQueryTermForMatch]; !alreadyCounted {
						numTyposForHit++
						uniqueMatchedOriginalQueryTokensTypos[originalQueryTermForMatch] = struct{}{}
					}
				} else { // Not a typo, check if it's an exact full word match
					if _, alreadyCounted := uniqueOriginalQueryTokensExact[originalQueryTermForMatch]; !alreadyCounted {
						// Check against the pre-tokenized full words of the field
						if fullWordsInField, fieldProcessed := docFullWordsByField[fieldName]; fieldProcessed {
							for _, docFullWord := range fullWordsInField {
								if originalQueryTermForMatch == docFullWord {
									numberExactWordsForHit++
									uniqueOriginalQueryTokensExact[originalQueryTermForMatch] = struct{}{}
									break // Found as exact match in this field for this originalQueryTermForMatch
								}
							}
						}
					}
				}
			}
			sort.Strings(matchedTermsResult[fieldName])
		}

		hitInfo := services.HitInfo{
			NumTypos:         numTyposForHit,
			NumberExactWords: numberExactWordsForHit,
			FilterScore:      ch.filterScore,
		}

		finalSelectHits = append(finalSelectHits, services.HitResult{
			Document:     s.filterDocumentFields(ch.doc, query.RetrievableFields),
			Score:        ch.score,
			FieldMatches: matchedTermsResult,
			Info:         hitInfo,
		})
	}

	// Sort finalSelectHits: Apply ranking criteria first, then by calculated score if no ranking criteria or as fallback
	sort.SliceStable(finalSelectHits, func(i, j int) bool {
		itemI := finalSelectHits[i]
		itemJ := finalSelectHits[j]

		docI := itemI.Document
		docJ := itemJ.Document

		// Apply ranking criteria first
		for _, criterion := range s.settings.RankingCriteria {
			// Special case: ~score means use the calculated search relevance score
			if criterion.Field == "~score" {
				if itemI.Score != itemJ.Score {
					if criterion.Order == "asc" {
						return itemI.Score < itemJ.Score
					} else {
						return itemI.Score > itemJ.Score
					}
				}
				continue // If scores are equal, continue to next criterion
			}

			// Special case: ~filters means use the filter matching score
			if criterion.Field == "~filters" {
				filterScoreI := itemI.Info.FilterScore
				filterScoreJ := itemJ.Info.FilterScore
				if filterScoreI != filterScoreJ {
					if criterion.Order == "asc" {
						return filterScoreI < filterScoreJ
					} else {
						return filterScoreI > filterScoreJ
					}
				}
				continue // If filter scores are equal, continue to next criterion
			}

			valI, okI := docI[criterion.Field]
			valJ, okJ := docJ[criterion.Field]

			if !okI && !okJ {
				continue
			}
			if okI && !okJ {
				return criterion.Order != "asc"
			}
			if !okI && okJ {
				return criterion.Order == "asc"
			}

			switch vI := valI.(type) {
			case string:
				if vJ, ok := valJ.(string); ok {
					if vI != vJ {
						if criterion.Order == "asc" {
							return vI < vJ
						} else {
							return vI > vJ
						}
					}
				}
			case float64:
				if vJ, ok := valJ.(float64); ok {
					if vI != vJ {
						if criterion.Order == "asc" {
							return vI < vJ
						} else {
							return vI > vJ
						}
					}
				}
			case int, int8, int16, int32, int64:
				fI, _ := convertToFloat64(vI)
				fJ, _ := convertToFloat64(valJ)
				if fI != fJ {
					if criterion.Order == "asc" {
						return fI < fJ
					} else {
						return fI > fJ
					}
				}
			case time.Time:
				if vJ, ok := valJ.(time.Time); ok {
					if !vI.Equal(vJ) {
						if criterion.Order == "asc" {
							return vI.Before(vJ)
						} else {
							return vI.After(vJ)
						}
					}
				}
			default:
				if strI, isStrI := valI.(string); isStrI {
					if strJ, isStrJ := valJ.(string); isStrJ {
						if criterion.Field == "ReleaseDate" { // Example specific field handling
							tI, errI := time.Parse(time.RFC3339Nano, strI)
							if errI != nil {
								tI, _ = time.Parse(time.RFC3339, strI)
							}
							tJ, errJ := time.Parse(time.RFC3339Nano, strJ)
							if errJ != nil {
								tJ, _ = time.Parse(time.RFC3339, strJ)
							}
							if tI.IsZero() || tJ.IsZero() {
								continue
							}
							if !tI.Equal(tJ) {
								if criterion.Order == "asc" {
									return tI.Before(tJ)
								} else {
									return tI.After(tJ)
								}
							}
						}
					}
				}
				continue
			}
		}

		// Fallback: if no ranking criteria resolved the comparison, sort by search score descending
		if itemI.Score != itemJ.Score {
			return itemI.Score > itemJ.Score
		}

		return false
	})

	// Apply deduplication if DistinctField is specified
	if s.settings.DistinctField != "" {
		finalSelectHits = s.deduplicateResults(finalSelectHits, s.settings.DistinctField)
	}

	totalHits := len(finalSelectHits)
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	var paginatedHits []services.HitResult
	if startIndex < totalHits {
		if endIndex > totalHits {
			endIndex = totalHits
		}
		paginatedHits = finalSelectHits[startIndex:endIndex]
	} else {
		paginatedHits = []services.HitResult{}
	}

	queryUUID := uuid.New().String()

	return services.SearchResult{
		Hits:     paginatedHits,
		Total:    totalHits,
		Page:     page,
		PageSize: pageSize,
		Took:     time.Since(startTime).Milliseconds(),
		QueryId:  queryUUID,
	}, nil
}

// deduplicateResults removes duplicate documents based on the specified field.
// It keeps the first occurrence (highest scoring) of each unique field value.
func (s *Service) deduplicateResults(hits []services.HitResult, distinctField string) []services.HitResult {
	if distinctField == "" || len(hits) == 0 {
		return hits
	}

	seen := make(map[string]bool)
	deduplicated := make([]services.HitResult, 0, len(hits))

	for _, hit := range hits {
		// Get the value of the distinct field from the document
		fieldValue, exists := hit.Document[distinctField]
		if !exists {
			// If the field doesn't exist, include the document (can't deduplicate)
			deduplicated = append(deduplicated, hit)
			continue
		}

		// Convert field value to string for comparison
		var fieldKey string
		switch v := fieldValue.(type) {
		case string:
			fieldKey = v
		case nil:
			fieldKey = ""
		default:
			fieldKey = fmt.Sprintf("%v", v)
		}

		// If we haven't seen this field value before, include it
		if !seen[fieldKey] {
			seen[fieldKey] = true
			deduplicated = append(deduplicated, hit)
		}
		// Otherwise, skip this duplicate
	}

	return deduplicated
}

// evaluateFilters evaluates a complex filter expression with AND/OR logic
func (s *Service) evaluateFilters(doc model.Document, expr services.Filters) (bool, float64) {
	// Handle individual filter conditions
	conditionResults := make([]bool, len(expr.Filters))
	conditionScores := make([]float64, len(expr.Filters))
	for i, condition := range expr.Filters {
		matches := s.evaluateFilterCondition(doc, condition)
		conditionResults[i] = matches
		if matches {
			conditionScores[i] = condition.Score
		}
	}

	// Handle nested groups
	groupResults := make([]bool, len(expr.Groups))
	groupScores := make([]float64, len(expr.Groups))
	for i, group := range expr.Groups {
		matches, score := s.evaluateFilters(doc, group)
		groupResults[i] = matches
		if matches {
			groupScores[i] = score
		}
	}

	// Combine all results based on operator
	allResults := append(conditionResults, groupResults...)
	allScores := append(conditionScores, groupScores...)

	if len(allResults) == 0 {
		return true, 0.0 // Empty expression matches with no score
	}

	switch strings.ToUpper(expr.Operator) {
	case "OR", "":
		// OR logic: at least one condition must match
		// Return score only from matching conditions
		totalScore := 0.0
		hasMatch := false
		for i, result := range allResults {
			if result {
				hasMatch = true
				totalScore += allScores[i]
			}
		}
		if hasMatch {
			return true, totalScore
		}
		return false, 0.0
	case "AND":
		// AND logic: all conditions must match
		// Return score from all conditions only if all match
		for _, result := range allResults {
			if !result {
				return false, 0.0
			}
		}
		// All matched, sum all scores
		totalScore := 0.0
		for _, score := range allScores {
			totalScore += score
		}
		return true, totalScore
	default:
		log.Printf("Warning: Unknown filter expression operator '%s', defaulting to OR", expr.Operator)
		// Default to OR logic
		totalScore := 0.0
		hasMatch := false
		for i, result := range allResults {
			if result {
				hasMatch = true
				totalScore += allScores[i]
			}
		}
		if hasMatch {
			return true, totalScore
		}
		return false, 0.0
	}
}

// evaluateFilterCondition evaluates a single filter condition
func (s *Service) evaluateFilterCondition(doc model.Document, condition services.FilterCondition) bool {
	filterableFieldsMap := make(map[string]struct{})
	for _, field := range s.settings.FilterableFields {
		filterableFieldsMap[field] = struct{}{}
	}

	fieldName := condition.Field
	operator := condition.Operator
	filterVal := condition.Value

	// If no operator specified, default to exact match for simple values or contains for arrays
	if operator == "" {
		// Auto-detect operator based on document field type
		if docFieldVal, exists := doc[fieldName]; exists {
			switch docFieldVal.(type) {
			case []string, []interface{}:
				operator = "_contains"
			default:
				operator = "_exact"
			}
		} else {
			operator = "_exact"
		}
	}

	if _, isFilterable := filterableFieldsMap[fieldName]; !isFilterable {
		log.Printf("Warning (Index: %s): Field '%s' in filter expression is not designated as filterable in settings, but will be evaluated.\n", s.settings.Name, fieldName)
	}

	docFieldValInterface, docFieldExists := doc[fieldName]
	if !docFieldExists {
		log.Printf("Warning (Index: %s, Field: %s): Field not found in document for filter condition. Criterion fails.\n", s.settings.Name, fieldName)
		return false
	}

	// Attempt type conversion for specific known types, e.g., dates stored as strings.
	var concreteDocFieldVal = docFieldValInterface
	if strings.Contains(strings.ToLower(fieldName), "date") {
		if strVal, ok := docFieldValInterface.(string); ok {
			tParsed, err := time.Parse(time.RFC3339Nano, strVal)
			if err != nil {
				tParsed, err = time.Parse(time.RFC3339, strVal)
			}
			if err == nil {
				concreteDocFieldVal = tParsed
			}
		}
	}

	return applyFilterLogic(concreteDocFieldVal, operator, filterVal, fieldName, s.settings.Name)
}

// convertToFloat64 converts various numeric types to float64
func convertToFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// applyFilterLogic applies the filter logic based on the operator for new filter expressions
func applyFilterLogic(docFieldVal interface{}, operator string, filterValue interface{}, fieldNameForDebug, indexNameForDebug string) bool {
	switch operator {
	case "", "_exact":
		return applyEqualityFilter(docFieldVal, filterValue)
	case "_ne":
		return !applyEqualityFilter(docFieldVal, filterValue)
	case "_gt":
		return applyComparisonFilter(docFieldVal, filterValue, "gt")
	case "_gte":
		return applyComparisonFilter(docFieldVal, filterValue, "gte")
	case "_lt":
		return applyComparisonFilter(docFieldVal, filterValue, "lt")
	case "_lte":
		return applyComparisonFilter(docFieldVal, filterValue, "lte")
	case "_contains":
		return applyContainsFilter(docFieldVal, filterValue)
	case "_ncontains":
		return !applyContainsFilter(docFieldVal, filterValue)
	case "_contains_any_of":
		return applyContainsAnyOfFilter(docFieldVal, filterValue)
	default:
		log.Printf("Warning: Unknown filter operator '%s' for field '%s' in index '%s'. Treating as equality.", operator, fieldNameForDebug, indexNameForDebug)
		return applyEqualityFilter(docFieldVal, filterValue)
	}
}

// applyEqualityFilter checks if two values are equal
func applyEqualityFilter(docFieldVal, filterValue interface{}) bool {
	// Handle array fields
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if compareValues(item, filterValue) {
				return true
			}
		}
		return false
	}

	return compareValues(docFieldVal, filterValue)
}

// applyComparisonFilter applies comparison operators (gt, gte, lt, lte)
func applyComparisonFilter(docFieldVal, filterValue interface{}, operator string) bool {
	// Handle array fields - check if any element satisfies the condition
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if compareValuesWithOperator(item, filterValue, operator) {
				return true
			}
		}
		return false
	}

	return compareValuesWithOperator(docFieldVal, filterValue, operator)
}

// applyContainsFilter checks if a field contains a value
func applyContainsFilter(docFieldVal, filterValue interface{}) bool {
	// Handle array fields - check if any element contains the filter value
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, item := range docArray {
			if itemStr, isStr := item.(string); isStr {
				if filterStr, isFilterStr := filterValue.(string); isFilterStr {
					if strings.Contains(strings.ToLower(itemStr), strings.ToLower(filterStr)) {
						return true
					}
				}
			}
		}
		return false
	}

	// Handle string array fields
	if docStrArray, isStrArray := docFieldVal.([]string); isStrArray {
		for _, item := range docStrArray {
			if filterStr, isFilterStr := filterValue.(string); isFilterStr {
				if strings.Contains(strings.ToLower(item), strings.ToLower(filterStr)) {
					return true
				}
			}
		}
		return false
	}

	// Handle single string field
	if docStr, isDocStr := docFieldVal.(string); isDocStr {
		if filterStr, isFilterStr := filterValue.(string); isFilterStr {
			return strings.Contains(strings.ToLower(docStr), strings.ToLower(filterStr))
		}
	}

	return false
}

// applyContainsAnyOfFilter checks if a field contains any of the provided values
func applyContainsAnyOfFilter(docFieldVal, filterValue interface{}) bool {
	// Filter value should be an array
	filterArray, isFilterArray := filterValue.([]interface{})
	if !isFilterArray {
		return false
	}

	// Handle array fields - check if any element matches any filter value
	if docArray, isArray := docFieldVal.([]interface{}); isArray {
		for _, docItem := range docArray {
			for _, filterItem := range filterArray {
				if compareValues(docItem, filterItem) {
					return true
				}
			}
		}
		return false
	}

	// Handle string array fields
	if docStrArray, isStrArray := docFieldVal.([]string); isStrArray {
		for _, docItem := range docStrArray {
			for _, filterItem := range filterArray {
				if compareValues(docItem, filterItem) {
					return true
				}
			}
		}
		return false
	}

	// Handle single field - check if it matches any filter value
	for _, filterItem := range filterArray {
		if compareValues(docFieldVal, filterItem) {
			return true
		}
	}

	return false
}

// compareValues compares two values for equality
func compareValues(docVal, filterVal interface{}) bool {
	// Direct equality check
	if docVal == filterVal {
		return true
	}

	// String comparison (case-sensitive)
	if docStr, isDocStr := docVal.(string); isDocStr {
		if filterStr, isFilterStr := filterVal.(string); isFilterStr {
			return docStr == filterStr
		}
	}

	// Numeric comparison
	if docFloat, docOk := convertToFloat64(docVal); docOk {
		if filterFloat, filterOk := convertToFloat64(filterVal); filterOk {
			return docFloat == filterFloat
		}
	}

	// Time comparison
	if docTime, docOk := convertToTime(docVal); docOk {
		if filterTime, filterOk := convertToTime(filterVal); filterOk {
			return docTime.Equal(filterTime)
		}
	}

	return false
}

// compareValuesWithOperator compares two values with a specific operator
func compareValuesWithOperator(docVal, filterVal interface{}, operator string) bool {
	// Numeric comparison
	if docFloat, docOk := convertToFloat64(docVal); docOk {
		if filterFloat, filterOk := convertToFloat64(filterVal); filterOk {
			switch operator {
			case "gt":
				return docFloat > filterFloat
			case "gte":
				return docFloat >= filterFloat
			case "lt":
				return docFloat < filterFloat
			case "lte":
				return docFloat <= filterFloat
			}
		}
	}

	// Time comparison
	if docTime, docOk := convertToTime(docVal); docOk {
		if filterTime, filterOk := convertToTime(filterVal); filterOk {
			switch operator {
			case "gt":
				return docTime.After(filterTime)
			case "gte":
				return docTime.After(filterTime) || docTime.Equal(filterTime)
			case "lt":
				return docTime.Before(filterTime)
			case "lte":
				return docTime.Before(filterTime) || docTime.Equal(filterTime)
			}
		}
	}

	// String comparison
	if docStr, isDocStr := docVal.(string); isDocStr {
		if filterStr, isFilterStr := filterVal.(string); isFilterStr {
			switch operator {
			case "gt":
				return docStr > filterStr
			case "gte":
				return docStr >= filterStr
			case "lt":
				return docStr < filterStr
			case "lte":
				return docStr <= filterStr
			}
		}
	}

	return false
}

// convertToTime converts various time representations to time.Time
func convertToTime(val interface{}) (time.Time, bool) {
	switch v := val.(type) {
	case time.Time:
		return v, true
	case string:
		// Try different time formats
		formats := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, true
			}
		}
	case int64:
		// Unix timestamp
		return time.Unix(v, 0), true
	case float64:
		// Unix timestamp as float
		return time.Unix(int64(v), 0), true
	}
	return time.Time{}, false
}

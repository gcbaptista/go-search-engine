package search

import (
	"fmt"
	"sort"
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

// candidateHit represents a document candidate during search processing
type candidateHit struct {
	doc                      model.Document
	score                    float64
	matchedQueryTermsByField map[string]map[string]struct{} // FieldName -> queryToken -> struct{}
}

// Search performs a search operation based on the query.
func (s *Service) Search(query services.SearchQuery) (services.SearchResult, error) {
	startTime := time.Now()

	// Determine effective searchable fields
	_, isFieldAllowed, err := s.determineSearchableFields(query)
	if err != nil {
		return services.SearchResult{}, err
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

	// Find document matches for each query token
	docMatchesByQueryToken := s.findDocumentMatches(originalQueryTokens, isFieldAllowed, query)

	// Collect candidate documents
	candidatesByDocID := s.collectCandidates(docMatchesByQueryToken, originalQueryTokens)

	// Apply filters
	filteredCandidates := s.applyFilters(candidatesByDocID, query.Filters)

	// Score and sort candidates
	scoredCandidates := s.scoreAndSortCandidates(filteredCandidates, originalQueryTokens)

	// Apply deduplication if needed
	if s.settings.DistinctField != "" {
		scoredCandidates = s.deduplicateCandidates(scoredCandidates, s.settings.DistinctField)
	}

	// Paginate results
	total := len(scoredCandidates)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedCandidates := scoredCandidates[start:end]

	// Convert to hit results
	hits := make([]services.HitResult, len(paginatedCandidates))
	for i, candidate := range paginatedCandidates {
		// Filter document fields if needed
		doc := s.filterDocumentFields(candidate.doc, query.RetrivableFields)

		// Convert matchedQueryTermsByField to FieldMatches format
		fieldMatches := make(map[string][]string)
		for fieldName, queryTerms := range candidate.matchedQueryTermsByField {
			terms := make([]string, 0, len(queryTerms))
			for term := range queryTerms {
				terms = append(terms, term)
			}
			if len(terms) > 0 {
				fieldMatches[fieldName] = terms
			}
		}

		hits[i] = services.HitResult{
			Document:     doc,
			Score:        candidate.score,
			FieldMatches: fieldMatches,
		}
	}

	queryUUID := uuid.New().String()
	return services.SearchResult{
		Hits:     hits,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Took:     time.Since(startTime).Milliseconds(),
		QueryId:  queryUUID,
	}, nil
}

// determineSearchableFields determines which fields can be searched based on query and settings
func (s *Service) determineSearchableFields(query services.SearchQuery) ([]string, func(string) bool, error) {
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
				return nil, nil, fmt.Errorf("restricted searchable field '%s' is not configured as a searchable field in index settings", restrictedField)
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

	return effectiveSearchableFields, isFieldAllowed, nil
}

// findDocumentMatches finds all document matches for the given query tokens
func (s *Service) findDocumentMatches(originalQueryTokens []string, isFieldAllowed func(string) bool, query services.SearchQuery) map[string]map[uint32][]index.PostingEntry {
	docMatchesByQueryToken := make(map[string]map[uint32][]index.PostingEntry)

	for _, queryToken := range originalQueryTokens {
		docMatchesByQueryToken[queryToken] = make(map[uint32][]index.PostingEntry)

		// 1. Exact matches for the queryToken
		if postingList, found := s.invertedIndex.Index[queryToken]; found {
			for _, entry := range postingList {
				if isFieldAllowed(entry.FieldName) {
					docMatchesByQueryToken[queryToken][entry.DocID] = append(docMatchesByQueryToken[queryToken][entry.DocID], entry)
				}
			}
		}

		// 2. Typo matches for the queryToken
		s.findTypoMatches(queryToken, docMatchesByQueryToken, isFieldAllowed, query)
	}

	return docMatchesByQueryToken
}

// findTypoMatches finds typo-tolerant matches for a query token
func (s *Service) findTypoMatches(queryToken string, docMatchesByQueryToken map[string]map[uint32][]index.PostingEntry, isFieldAllowed func(string) bool, query services.SearchQuery) {
	// Check if this query token is in the non-typo tolerant words list
	isNonTypoTolerant := false
	for _, nonTypoWord := range s.settings.NonTypoTolerantWords {
		if strings.EqualFold(queryToken, nonTypoWord) {
			isNonTypoTolerant = true
			break
		}
	}

	// Skip typo matching if this word is in the non-typo tolerant list
	if isNonTypoTolerant {
		return
	}

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

	// 1-typo matches
	if minWordSizeFor1Typo > 0 && len(queryToken) >= minWordSizeFor1Typo {
		typos1 := s.typoFinder.GenerateTyposWithTimeLimit(queryToken, 1, maxTypoResults, timeLimit)
		s.processTypoMatches(typos1, queryToken, docMatchesByQueryToken, isFieldAllowed)
	}

	// 2-typo matches
	if minWordSizeFor2Typos > 0 && len(queryToken) >= minWordSizeFor2Typos {
		typos2 := s.typoFinder.GenerateTyposWithTimeLimit(queryToken, 2, maxTypoResults, timeLimit)
		s.processTypoMatches(typos2, queryToken, docMatchesByQueryToken, isFieldAllowed)
	}
}

// processTypoMatches processes typo matches and adds them to the results
func (s *Service) processTypoMatches(typos []string, queryToken string, docMatchesByQueryToken map[string]map[uint32][]index.PostingEntry, isFieldAllowed func(string) bool) {
	for _, typoTerm := range typos {
		// Check if the typo term itself is in the non-typo tolerant words list
		isTypoTermNonTypoTolerant := false
		for _, nonTypoWord := range s.settings.NonTypoTolerantWords {
			if strings.EqualFold(typoTerm, nonTypoWord) {
				isTypoTermNonTypoTolerant = true
				break
			}
			// Also check if the typo term is a prefix of a non-typo tolerant word
			if len(typoTerm) < len(nonTypoWord) && strings.HasPrefix(strings.ToLower(nonTypoWord), strings.ToLower(typoTerm)) {
				isTypoTermNonTypoTolerant = true
				break
			}
		}

		if isTypoTermNonTypoTolerant {
			continue
		}

		if postingList, found := s.invertedIndex.Index[typoTerm]; found {
			for _, entry := range postingList {
				if isFieldAllowed(entry.FieldName) {
					docMatchesByQueryToken[queryToken][entry.DocID] = append(docMatchesByQueryToken[queryToken][entry.DocID], entry)
				}
			}
		}
	}
}

// collectCandidates collects all candidate documents from the matches
func (s *Service) collectCandidates(docMatchesByQueryToken map[string]map[uint32][]index.PostingEntry, originalQueryTokens []string) map[uint32]*candidateHit {
	candidatesByDocID := make(map[uint32]*candidateHit)

	for _, queryToken := range originalQueryTokens {
		for docID, entries := range docMatchesByQueryToken[queryToken] {
			if candidatesByDocID[docID] == nil {
				doc, exists := s.documentStore.Docs[docID]
				if !exists {
					continue
				}
				candidatesByDocID[docID] = &candidateHit{
					doc:                      doc,
					score:                    0,
					matchedQueryTermsByField: make(map[string]map[string]struct{}),
				}
			}

			candidate := candidatesByDocID[docID]
			for _, entry := range entries {
				if candidate.matchedQueryTermsByField[entry.FieldName] == nil {
					candidate.matchedQueryTermsByField[entry.FieldName] = make(map[string]struct{})
				}
				candidate.matchedQueryTermsByField[entry.FieldName][queryToken] = struct{}{}
			}
		}
	}

	return candidatesByDocID
}

// scoreAndSortCandidates scores candidates and sorts them by relevance
func (s *Service) scoreAndSortCandidates(candidatesByDocID map[uint32]*candidateHit, originalQueryTokens []string) []*candidateHit {
	candidates := make([]*candidateHit, 0, len(candidatesByDocID))
	for _, candidate := range candidatesByDocID {
		// Calculate score based on term frequency and field matches
		candidate.score = s.calculateScore(candidate, originalQueryTokens)
		candidates = append(candidates, candidate)
	}

	// Sort by score (descending) and then by document for consistency
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		// Use a consistent field for tie-breaking (assuming documents have an ID field)
		docI := candidates[i].doc
		docJ := candidates[j].doc
		if idI, okI := docI["id"]; okI {
			if idJ, okJ := docJ["id"]; okJ {
				return fmt.Sprintf("%v", idI) < fmt.Sprintf("%v", idJ)
			}
		}
		return false
	})

	return candidates
}

// calculateScore calculates the relevance score for a candidate document
func (s *Service) calculateScore(candidate *candidateHit, originalQueryTokens []string) float64 {
	score := 0.0

	// Count matched query terms
	matchedTerms := 0
	for _, queryToken := range originalQueryTokens {
		for fieldName := range candidate.matchedQueryTermsByField {
			if _, found := candidate.matchedQueryTermsByField[fieldName][queryToken]; found {
				matchedTerms++
				score += 1.0 // Base score for each matched term
				break        // Count each query term only once per document
			}
		}
	}

	// Boost score based on the number of matched terms
	if len(originalQueryTokens) > 0 {
		termMatchRatio := float64(matchedTerms) / float64(len(originalQueryTokens))
		score *= termMatchRatio
	}

	// Apply field-based scoring if configured
	for fieldName := range candidate.matchedQueryTermsByField {
		// You could add field-specific boosts here based on settings
		_ = fieldName
	}

	return score
}

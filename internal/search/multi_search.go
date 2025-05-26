package search

import (
	"context"
	"fmt"
	"time"

	"github.com/gcbaptista/go-search-engine/services"
)

// MultiSearch executes multiple named search queries in parallel
func (s *Service) MultiSearch(ctx context.Context, multiQuery services.MultiSearchQuery) (*services.MultiSearchResult, error) {
	startTime := time.Now()

	if len(multiQuery.Queries) == 0 {
		return nil, fmt.Errorf("at least one query is required")
	}

	// Create channels for parallel execution
	type queryResult struct {
		name   string
		result services.SearchResult
		err    error
	}

	resultChan := make(chan queryResult, len(multiQuery.Queries))

	// Execute queries in parallel
	for _, namedQuery := range multiQuery.Queries {
		if namedQuery.Name == "" {
			return nil, fmt.Errorf("each query must have a non-empty name")
		}

		// Launch goroutine for each query
		go func(nq services.NamedSearchQuery) {
			// Convert NamedSearchQuery to SearchQuery
			searchQuery := services.SearchQuery{
				QueryString:              nq.Query,
				RestrictSearchableFields: nq.RestrictSearchableFields,
				RetrivableFields:         nq.RetrivableFields,
				Filters:                  nq.Filters,
				FilterExpression:         nq.FilterExpression,
				Page:                     multiQuery.Page,
				PageSize:                 multiQuery.PageSize,
				MinWordSizeFor1Typo:      nq.MinWordSizeFor1Typo,
				MinWordSizeFor2Typos:     nq.MinWordSizeFor2Typos,
			}

			// Execute the search
			result, err := s.Search(searchQuery)

			// Send result to channel
			resultChan <- queryResult{
				name:   nq.Name,
				result: result,
				err:    err,
			}
		}(namedQuery)
	}

	// Collect results from all goroutines
	results := make(map[string]services.SearchResult)
	for i := 0; i < len(multiQuery.Queries); i++ {
		select {
		case qr := <-resultChan:
			if qr.err != nil {
				return nil, fmt.Errorf("error executing query '%s': %w", qr.name, qr.err)
			}
			results[qr.name] = qr.result
		case <-ctx.Done():
			return nil, fmt.Errorf("multi-search cancelled: %w", ctx.Err())
		}
	}

	processingTime := time.Since(startTime)

	return &services.MultiSearchResult{
		Results:          results,
		TotalQueries:     len(multiQuery.Queries),
		ProcessingTimeMs: float64(processingTime.Nanoseconds()) / 1e6,
	}, nil
}

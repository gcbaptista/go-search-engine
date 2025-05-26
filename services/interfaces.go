package services

import (
	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/model"
)

// HitInfo contains metadata about a search hit, like typo counts and exact matches.
// This will be embedded in HitResult.
type HitInfo struct {
	NumTypos         int     `json:"num_typos"`          // Number of original query terms that matched via typo correction
	NumberExactWords int     `json:"number_exact_words"` // Number of original query terms that matched exactly (not via typo)
	FilterScore      float64 `json:"filter_score"`       // Score from filter expression matching
}

// HitResult represents a single document in the search results,
// including the document itself and details about which query terms matched in which fields.
type HitResult struct {
	Document     model.Document      `json:"document"`
	FieldMatches map[string][]string `json:"field_matches"` // e.g., {"title": ["lord", "ring"], "tags": ["epic"]}
	Score        float64             `json:"score"`         // The overall score for this hit
	Info         HitInfo             `json:"hit_info"`      // Contains metadata like typo counts and exact matches
}

type SearchResult struct {
	Hits     []HitResult `json:"hits"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Took     int64       `json:"took"`     // milliseconds
	QueryId  string      `json:"query_id"` // unique UUID for this search query
}

type SearchQuery struct {
	QueryString              string
	FilterExpression         *FilterExpression `json:"filter_expression,omitempty"` // Complex filter expressions
	Page                     int
	PageSize                 int
	RestrictSearchableFields []string `json:"restrict_searchable_fields,omitempty"` // Optional: subset of searchable fields to search in
	RetrivableFields         []string `json:"retrivable_fields,omitempty"`          // Optional: subset of document fields to return in results
	MinWordSizeFor1Typo      *int     `json:"min_word_size_for_1_typo,omitempty"`   // Optional: override index setting for minimum word size for 1 typo
	MinWordSizeFor2Typos     *int     `json:"min_word_size_for_2_typos,omitempty"`  // Optional: override index setting for minimum word size for 2 typos
}

// MultiSearchQuery represents a request to execute multiple named search queries
type MultiSearchQuery struct {
	Queries  []NamedSearchQuery `json:"queries"`
	Page     int                `json:"page,omitempty"`
	PageSize int                `json:"page_size,omitempty"`
}

// NamedSearchQuery represents a single named search query within a multi-search request
type NamedSearchQuery struct {
	Name                     string            `json:"name"`
	Query                    string            `json:"query"`
	RestrictSearchableFields []string          `json:"restrict_searchable_fields,omitempty"`
	RetrivableFields         []string          `json:"retrivable_fields,omitempty"`
	FilterExpression         *FilterExpression `json:"filter_expression,omitempty"`
	MinWordSizeFor1Typo      *int              `json:"min_word_size_for_1_typo,omitempty"`
	MinWordSizeFor2Typos     *int              `json:"min_word_size_for_2_typos,omitempty"`
}

// MultiSearchResult represents the response from a multi-search operation
type MultiSearchResult struct {
	Results          map[string]SearchResult `json:"results"`
	TotalQueries     int                     `json:"total_queries"`
	ProcessingTimeMs float64                 `json:"processing_time_ms"`
}

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Score    float64     `json:"score,omitempty"` // Optional score boost for matching this condition
}

// FilterExpression represents a complex filter expression with AND/OR logic
type FilterExpression struct {
	Operator string             `json:"operator"` // "AND" or "OR"
	Filters  []FilterCondition  `json:"filters"`
	Groups   []FilterExpression `json:"groups"` // Nested filter expressions
}

// Indexer defines operations for adding data to an index
type Indexer interface {
	AddDocuments(docs []model.Document) error
	DeleteAllDocuments() error
	DeleteDocument(docID string) error
}

// Searcher defines operations for querying an index
type Searcher interface {
	Search(query SearchQuery) (SearchResult, error)
}

// MultiSearcher defines operations for performing multiple queries in a single request
type MultiSearcher interface {
	MultiSearch(query MultiSearchQuery) (*MultiSearchResult, error)
}

// IndexManager manages the lifecycle of indices
type IndexManager interface {
	CreateIndex(settings config.IndexSettings) error
	GetIndex(name string) (IndexAccessor, error) // IndexAccessor combines Indexer and Searcher
	GetIndexSettings(name string) (config.IndexSettings, error)
	UpdateIndexSettings(name string, settings config.IndexSettings) error
	RenameIndex(oldName, newName string) error
	DeleteIndex(name string) error
	ListIndexes() []string
	PersistIndexData(indexName string) error
}

// IndexManagerWithReindex extends IndexManager with reindexing capabilities for settings updates
type IndexManagerWithReindex interface {
	IndexManager
	UpdateIndexSettingsWithReindex(name string, settings config.IndexSettings) error
}

// IndexManagerWithAsyncReindex extends IndexManager with async reindexing capabilities
type IndexManagerWithAsyncReindex interface {
	IndexManager
	UpdateIndexSettingsWithAsyncReindex(name string, settings config.IndexSettings) (string, error) // Returns job ID
}

// JobManager defines operations for managing background jobs
type JobManager interface {
	GetJob(jobID string) (*model.Job, error)
	ListJobs(indexName string, status *model.JobStatus) []*model.Job
}

type IndexAccessor interface {
	Indexer
	Searcher
	MultiSearcher
	Settings() config.IndexSettings
}

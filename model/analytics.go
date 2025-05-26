package model

import "time"

// SearchEvent represents a single search event for analytics tracking
type SearchEvent struct {
	IndexName    string        `json:"index_name"`
	Query        string        `json:"query"`
	SearchType   string        `json:"search_type"` // "exact_match", "fuzzy_search", "filtered", "wildcard"
	ResponseTime time.Duration `json:"response_time"`
	ResultCount  int           `json:"result_count"`
	Timestamp    time.Time     `json:"timestamp"`
}

// PopularSearch represents aggregated data for popular search terms
type PopularSearch struct {
	Query       string `json:"query"`
	SearchCount int    `json:"search_count"`
	TrendChange string `json:"trend_change,omitempty"` // "up", "down", "stable"
}

// IndexStats represents statistics for a specific index
type IndexStats struct {
	IndexName     string  `json:"index_name"`
	DocumentCount int     `json:"document_count"`
	SearchCount   int     `json:"search_count"`
	SizeInMB      float64 `json:"size_mb"`
}

// ResponseTimeDistribution represents response time distribution buckets
type ResponseTimeDistribution struct {
	Bucket0To25ms     int     `json:"bucket_0_25ms"`
	Bucket25To50ms    int     `json:"bucket_25_50ms"`
	Bucket50To100ms   int     `json:"bucket_50_100ms"`
	Bucket100msPlus   int     `json:"bucket_100ms_plus"`
	Percentage0To25   float64 `json:"percentage_0_25"`
	Percentage25To50  float64 `json:"percentage_25_50"`
	Percentage50To100 float64 `json:"percentage_50_100"`
	Percentage100Plus float64 `json:"percentage_100_plus"`
}

// SearchTypeStats represents statistics for different search types
type SearchTypeStats struct {
	ExactMatch  int `json:"exact_match"`
	FuzzySearch int `json:"fuzzy_search"`
	Filtered    int `json:"filtered"`
	Wildcard    int `json:"wildcard"`
}

// SearchPerformanceHourly represents hourly search performance data
type SearchPerformanceHourly struct {
	Hour            int   `json:"hour"`
	SearchCount     int   `json:"search_count"`
	AvgResponseTime int64 `json:"avg_response_time"` // in milliseconds
}

// SystemHealth represents system health metrics
type SystemHealth struct {
	MemoryUsage float64 `json:"memory_usage_percent"`
	CPUUsage    float64 `json:"cpu_usage_percent"`
	DiskSpace   float64 `json:"disk_space_percent"`
	IndexHealth float64 `json:"index_health_percent"`
}

// AnalyticsDashboard represents the complete analytics dashboard data
type AnalyticsDashboard struct {
	// Summary metrics
	TotalSearches         int     `json:"total_searches"`
	SearchesChangePercent float64 `json:"searches_change_percent"`
	AvgResponseTime       int64   `json:"avg_response_time"` // in milliseconds
	ResponseTimeChange    string  `json:"response_time_change"`
	TotalDocuments        int     `json:"total_documents"`
	DocumentsChangeCount  int     `json:"documents_change_count"`
	ActiveIndexes         int     `json:"active_indexes"`
	IndexesChangeCount    int     `json:"indexes_change_count"`

	// Detailed analytics
	SearchPerformance24h     []SearchPerformanceHourly `json:"search_performance_24h"`
	PopularSearches          []PopularSearch           `json:"popular_searches"`
	IndexUsage               []IndexStats              `json:"index_usage"`
	ResponseTimeDistribution ResponseTimeDistribution  `json:"response_time_distribution"`
	SearchTypes              SearchTypeStats           `json:"search_types"`
	SystemHealth             SystemHealth              `json:"system_health"`
}

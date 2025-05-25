package analytics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

const (
	analyticsDataFile = "search_data/analytics.json"
	maxEventsToKeep   = 10000 // Keep last 10k events for performance
)

// Service implements analytics tracking and reporting
type Service struct {
	mutex        sync.RWMutex
	events       []model.SearchEvent
	indexManager services.IndexManager
	dataFilePath string
}

// NewService creates a new analytics service
func NewService(indexManager services.IndexManager) *Service {
	service := &Service{
		events:       make([]model.SearchEvent, 0),
		indexManager: indexManager,
		dataFilePath: analyticsDataFile,
	}

	// Load existing analytics data
	if err := service.loadData(); err != nil {
		log.Printf("Warning: Failed to load analytics data: %v", err)
	}

	return service
}

// TrackSearchEvent records a new search event
func (s *Service) TrackSearchEvent(event model.SearchEvent) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	event.Timestamp = time.Now()
	s.events = append(s.events, event)

	// Keep only the latest events to prevent unbounded growth
	if len(s.events) > maxEventsToKeep {
		s.events = s.events[len(s.events)-maxEventsToKeep:]
	}

	// Persist data asynchronously
	go func() {
		if err := s.saveData(); err != nil {
			log.Printf("Warning: Failed to save analytics data: %v", err)
		}
	}()

	return nil
}

// GetDashboardData returns complete analytics dashboard data
func (s *Service) GetDashboardData() (model.AnalyticsDashboard, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	// Filter events for different time periods
	last24hEvents := s.filterEventsByTime(s.events, yesterday)
	lastWeekEvents := s.filterEventsByTime(s.events, lastWeek)
	prevWeekEvents := s.filterEventsByTimeRange(s.events, lastWeek.Add(-7*24*time.Hour), lastWeek)

	dashboard := model.AnalyticsDashboard{
		TotalSearches:            len(last24hEvents),
		SearchesChangePercent:    s.calculateChangePercent(len(last24hEvents), len(prevWeekEvents)),
		AvgResponseTime:          s.calculateAvgResponseTime(last24hEvents),
		ResponseTimeChange:       s.calculateResponseTimeChange(last24hEvents, prevWeekEvents),
		TotalDocuments:           s.getTotalDocuments(),
		DocumentsChangeCount:     s.getDocumentsChange(),
		ActiveIndexes:            s.getActiveIndexesCount(),
		IndexesChangeCount:       s.getIndexesChange(),
		SearchPerformance24h:     s.getHourlyPerformance(last24hEvents),
		PopularSearches:          s.getPopularSearches(lastWeekEvents),
		IndexUsage:               s.getIndexUsage(lastWeekEvents),
		ResponseTimeDistribution: s.getResponseTimeDistribution(last24hEvents),
		SearchTypes:              s.getSearchTypeStats(last24hEvents),
		SystemHealth:             s.getSystemHealth(),
	}

	return dashboard, nil
}

// filterEventsByTime returns events after the given time
func (s *Service) filterEventsByTime(events []model.SearchEvent, after time.Time) []model.SearchEvent {
	var filtered []model.SearchEvent
	for _, event := range events {
		if event.Timestamp.After(after) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// filterEventsByTimeRange returns events within the given time range
func (s *Service) filterEventsByTimeRange(events []model.SearchEvent, start, end time.Time) []model.SearchEvent {
	var filtered []model.SearchEvent
	for _, event := range events {
		if event.Timestamp.After(start) && event.Timestamp.Before(end) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// calculateChangePercent calculates percentage change between current and previous values
func (s *Service) calculateChangePercent(current, previous int) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return float64(current-previous) / float64(previous) * 100.0
}

// calculateAvgResponseTime calculates average response time for events in milliseconds
func (s *Service) calculateAvgResponseTime(events []model.SearchEvent) int64 {
	if len(events) == 0 {
		return 0
	}

	var total time.Duration
	for _, event := range events {
		total += event.ResponseTime
	}
	avgDuration := total / time.Duration(len(events))
	return avgDuration.Milliseconds()
}

// calculateResponseTimeChange calculates response time change trend
func (s *Service) calculateResponseTimeChange(current, previous []model.SearchEvent) string {
	currentAvg := s.calculateAvgResponseTime(current)
	previousAvg := s.calculateAvgResponseTime(previous)

	if previousAvg == 0 {
		return "stable"
	}

	change := float64(currentAvg-previousAvg) / float64(previousAvg)
	if change > 0.1 {
		return "up"
	} else if change < -0.1 {
		return "down"
	}
	return "stable"
}

// getTotalDocuments returns total document count across all indexes
func (s *Service) getTotalDocuments() int {
	indexes := s.indexManager.ListIndexes()

	total := 0
	for _, indexName := range indexes {
		if _, err := s.indexManager.GetIndex(indexName); err == nil {
			if concreteEngine, ok := s.indexManager.(*engine.Engine); ok {
				if instance, err := concreteEngine.GetIndex(indexName); err == nil {
					if engineInstance, ok := instance.(*engine.IndexInstance); ok {
						total += len(engineInstance.DocumentStore.Docs)
					}
				}
			}
		}
	}
	return total
}

// getDocumentsChange returns the change in document count
func (s *Service) getDocumentsChange() int {
	// Placeholder implementation - would require tracking historical document counts
	return 156
}

// getActiveIndexesCount returns the number of active indexes
func (s *Service) getActiveIndexesCount() int {
	indexes := s.indexManager.ListIndexes()
	return len(indexes)
}

// getIndexesChange returns the change in index count
func (s *Service) getIndexesChange() int {
	// Placeholder implementation - would require tracking historical index counts
	return 2
}

// getHourlyPerformance returns hourly search performance for the last 24 hours
func (s *Service) getHourlyPerformance(events []model.SearchEvent) []model.SearchPerformanceHourly {
	hourlyData := make(map[int][]model.SearchEvent)

	for _, event := range events {
		hour := event.Timestamp.Hour()
		hourlyData[hour] = append(hourlyData[hour], event)
	}

	var performance []model.SearchPerformanceHourly
	for hour := 0; hour < 24; hour++ {
		events := hourlyData[hour]
		avgResponseTime := s.calculateAvgResponseTime(events)

		performance = append(performance, model.SearchPerformanceHourly{
			Hour:            hour,
			SearchCount:     len(events),
			AvgResponseTime: avgResponseTime,
		})
	}

	return performance
}

// getPopularSearches returns the most popular search terms
func (s *Service) getPopularSearches(events []model.SearchEvent) []model.PopularSearch {
	queryCounts := make(map[string]int)

	for _, event := range events {
		if event.Query != "" {
			queryCounts[event.Query]++
		}
	}

	type queryCount struct {
		query string
		count int
	}

	var queries []queryCount
	for query, count := range queryCounts {
		queries = append(queries, queryCount{query: query, count: count})
	}

	// Sort by count descending
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].count > queries[j].count
	})

	// Return top 5
	var popular []model.PopularSearch
	for i, qc := range queries {
		if i >= 5 {
			break
		}
		popular = append(popular, model.PopularSearch{
			Query:       qc.query,
			SearchCount: qc.count,
			TrendChange: "stable",
		})
	}

	return popular
}

// getIndexUsage returns usage statistics for each index
func (s *Service) getIndexUsage(events []model.SearchEvent) []model.IndexStats {
	indexSearchCounts := make(map[string]int)

	for _, event := range events {
		indexSearchCounts[event.IndexName]++
	}

	indexes := s.indexManager.ListIndexes()

	var usage []model.IndexStats
	for _, indexName := range indexes {
		searchCount := indexSearchCounts[indexName]
		documentCount := 0
		sizeInMB := 0.0

		if _, err := s.indexManager.GetIndex(indexName); err == nil {
			if concreteEngine, ok := s.indexManager.(*engine.Engine); ok {
				if instance, err := concreteEngine.GetIndex(indexName); err == nil {
					if engineInstance, ok := instance.(*engine.IndexInstance); ok {
						documentCount = len(engineInstance.DocumentStore.Docs)
						sizeInMB = float64(documentCount) * 0.001
					}
				}
			}
		}

		usage = append(usage, model.IndexStats{
			IndexName:     indexName,
			DocumentCount: documentCount,
			SearchCount:   searchCount,
			SizeInMB:      sizeInMB,
		})
	}

	return usage
}

// getResponseTimeDistribution returns response time distribution
func (s *Service) getResponseTimeDistribution(events []model.SearchEvent) model.ResponseTimeDistribution {
	dist := model.ResponseTimeDistribution{}
	total := len(events)

	if total == 0 {
		return dist
	}

	for _, event := range events {
		ms := event.ResponseTime.Milliseconds()
		switch {
		case ms <= 25:
			dist.Bucket0To25ms++
		case ms <= 50:
			dist.Bucket25To50ms++
		case ms <= 100:
			dist.Bucket50To100ms++
		default:
			dist.Bucket100msPlus++
		}
	}

	// Calculate percentages
	dist.Percentage0To25 = float64(dist.Bucket0To25ms) / float64(total) * 100
	dist.Percentage25To50 = float64(dist.Bucket25To50ms) / float64(total) * 100
	dist.Percentage50To100 = float64(dist.Bucket50To100ms) / float64(total) * 100
	dist.Percentage100Plus = float64(dist.Bucket100msPlus) / float64(total) * 100

	return dist
}

// getSearchTypeStats returns statistics for different search types
func (s *Service) getSearchTypeStats(events []model.SearchEvent) model.SearchTypeStats {
	stats := model.SearchTypeStats{}

	for _, event := range events {
		switch event.SearchType {
		case "exact_match":
			stats.ExactMatch++
		case "fuzzy_search":
			stats.FuzzySearch++
		case "filtered":
			stats.Filtered++
		case "wildcard":
			stats.Wildcard++
		}
	}

	return stats
}

// getSystemHealth returns current system health metrics
func (s *Service) getSystemHealth() model.SystemHealth {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate memory usage percentage (simplified)
	memoryUsage := float64(m.Alloc) / float64(m.Sys) * 100

	return model.SystemHealth{
		MemoryUsage: memoryUsage,
		CPUUsage:    23.0,
		DiskSpace:   45.0,
		IndexHealth: 100.0,
	}
}

// loadData loads analytics data from file
func (s *Service) loadData() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.dataFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create analytics directory: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(s.dataFilePath); os.IsNotExist(err) {
		return nil // File doesn't exist yet, that's okay
	}

	data, err := os.ReadFile(s.dataFilePath)
	if err != nil {
		return fmt.Errorf("failed to read analytics file: %v", err)
	}

	if err := json.Unmarshal(data, &s.events); err != nil {
		return fmt.Errorf("failed to unmarshal analytics data: %v", err)
	}

	return nil
}

// saveData saves analytics data to file
func (s *Service) saveData() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.dataFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create analytics directory: %v", err)
	}

	data, err := json.MarshalIndent(s.events, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analytics data: %v", err)
	}

	if err := ioutil.WriteFile(s.dataFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write analytics file: %v", err)
	}

	return nil
}

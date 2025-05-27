package analytics

import (
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// MockIndexManager is a simple mock for testing
type MockIndexManager struct {
	indexes []string
}

func (m *MockIndexManager) CreateIndex(_ config.IndexSettings) error          { return nil }
func (m *MockIndexManager) GetIndex(_ string) (services.IndexAccessor, error) { return nil, nil }
func (m *MockIndexManager) GetIndexSettings(_ string) (config.IndexSettings, error) {
	return config.IndexSettings{}, nil
}
func (m *MockIndexManager) UpdateIndexSettings(_ string, _ config.IndexSettings) error {
	return nil
}
func (m *MockIndexManager) RenameIndex(_, _ string) error   { return nil }
func (m *MockIndexManager) DeleteIndex(_ string) error      { return nil }
func (m *MockIndexManager) ListIndexes() []string           { return m.indexes }
func (m *MockIndexManager) PersistIndexData(_ string) error { return nil }

func TestAnalyticsService_TrackSearchEvent(t *testing.T) {
	mockIndexManager := &MockIndexManager{
		indexes: []string{"test_index"},
	}

	service := NewService(mockIndexManager)
	// Clear any existing events from previous tests
	service.events = make([]model.SearchEvent, 0)

	event := model.SearchEvent{
		IndexName:    "test_index",
		Query:        "test query",
		SearchType:   "exact_match",
		ResponseTime: 50 * time.Millisecond,
		ResultCount:  10,
	}

	err := service.TrackSearchEvent(event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify event was stored
	if len(service.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(service.events))
	}

	storedEvent := service.events[0]
	if storedEvent.IndexName != event.IndexName {
		t.Errorf("Expected IndexName %s, got %s", event.IndexName, storedEvent.IndexName)
	}
	if storedEvent.Query != event.Query {
		t.Errorf("Expected Query %s, got %s", event.Query, storedEvent.Query)
	}
}

func TestAnalyticsService_GetDashboardData(t *testing.T) {
	mockIndexManager := &MockIndexManager{
		indexes: []string{"test_index1", "test_index2"},
	}

	service := NewService(mockIndexManager)
	// Clear any existing events from previous tests
	service.events = make([]model.SearchEvent, 0)

	// Add some test events
	events := []model.SearchEvent{
		{
			IndexName:    "test_index1",
			Query:        "matrix",
			SearchType:   "exact_match",
			ResponseTime: 30 * time.Millisecond,
			ResultCount:  5,
			Timestamp:    time.Now().Add(-1 * time.Hour),
		},
		{
			IndexName:    "test_index2",
			Query:        "batman",
			SearchType:   "fuzzy_search",
			ResponseTime: 45 * time.Millisecond,
			ResultCount:  3,
			Timestamp:    time.Now().Add(-2 * time.Hour),
		},
	}

	for _, event := range events {
		if err := service.TrackSearchEvent(event); err != nil {
			t.Fatalf("Failed to track search event: %v", err)
		}
	}

	dashboard, err := service.GetDashboardData()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Basic validation
	if dashboard.ActiveIndexes != 2 {
		t.Errorf("Expected 2 active indexes, got %d", dashboard.ActiveIndexes)
	}

	if len(dashboard.SearchPerformance24h) != 24 {
		t.Errorf("Expected 24 hourly performance entries, got %d", len(dashboard.SearchPerformance24h))
	}

	if len(dashboard.PopularSearches) == 0 {
		t.Error("Expected some popular searches, got none")
	}
}

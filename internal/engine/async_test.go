package engine

import (
	"os"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

func TestEngine_UpdateIndexSettingsWithAsyncReindex(t *testing.T) {
	// Create a temporary directory for test
	testDir := createTestDir(t)
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to remove test directory: %v", err)
		}
	}()

	engine := NewEngine(testDir)
	defer engine.jobManager.Stop()

	// Create a test index
	settings := config.IndexSettings{
		Name:                 "test-async-index",
		SearchableFields:     []string{"title", "description"},
		FilterableFields:     []string{"category"},
		MinWordSizeFor1Typo:  4,
		MinWordSizeFor2Typos: 8,
	}

	err := engine.CreateIndex(settings)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}

	// Add some test documents
	indexAccessor, err := engine.GetIndex("test-async-index")
	if err != nil {
		t.Fatalf("Failed to get test index: %v", err)
	}

	testDocs := []model.Document{
		{"documentID": "1", "title": "Test Document 1", "description": "First test document", "category": "test"},
		{"documentID": "2", "title": "Test Document 2", "description": "Second test document", "category": "test"},
		{"documentID": "3", "title": "Test Document 3", "description": "Third test document", "category": "test"},
	}

	err = indexAccessor.AddDocuments(testDocs)
	if err != nil {
		t.Fatalf("Failed to add test documents: %v", err)
	}

	// Update settings asynchronously (change searchable fields - requires reindexing)
	newSettings := settings
	newSettings.SearchableFields = []string{"title", "description", "category"}

	jobID, err := engine.UpdateIndexSettingsWithAsyncReindex("test-async-index", newSettings)
	if err != nil {
		t.Fatalf("Failed to start async reindexing: %v", err)
	}

	if jobID == "" {
		t.Error("Expected non-empty job ID")
	}

	// Check initial job status
	job, err := engine.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if job.Type != model.JobTypeReindex {
		t.Errorf("Expected job type %s, got %s", model.JobTypeReindex, job.Type)
	}

	if job.IndexName != "test-async-index" {
		t.Errorf("Expected index name 'test-async-index', got %s", job.IndexName)
	}

	// Wait for job completion (with timeout)
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var finalJob *model.Job
	for {
		select {
		case <-timeout:
			t.Fatal("Job did not complete within timeout")
		case <-ticker.C:
			finalJob, err = engine.GetJob(jobID)
			if err != nil {
				t.Fatalf("Failed to get job status: %v", err)
			}

			if finalJob.Status == model.JobStatusCompleted {
				goto jobCompleted
			}

			if finalJob.Status == model.JobStatusFailed {
				t.Fatalf("Job failed: %s", finalJob.Error)
			}

			// Log progress for debugging
			if finalJob.Progress != nil {
				t.Logf("Job progress: %d/%d - %s",
					finalJob.Progress.Current,
					finalJob.Progress.Total,
					finalJob.Progress.Message)
			}
		}
	}

jobCompleted:
	// Verify job completed successfully
	if finalJob.Status != model.JobStatusCompleted {
		t.Errorf("Expected job status %s, got %s", model.JobStatusCompleted, finalJob.Status)
	}

	if finalJob.CompletedAt == nil {
		t.Error("Expected completion timestamp to be set")
	}

	// Verify settings were updated
	updatedSettings, err := engine.GetIndexSettings("test-async-index")
	if err != nil {
		t.Fatalf("Failed to get updated settings: %v", err)
	}

	if len(updatedSettings.SearchableFields) != 3 {
		t.Errorf("Expected 3 searchable fields, got %d", len(updatedSettings.SearchableFields))
	}

	// Verify documents are still searchable
	updatedAccessor, err := engine.GetIndex("test-async-index")
	if err != nil {
		t.Fatalf("Failed to get updated index: %v", err)
	}

	// Search to verify reindexing worked
	searchQuery := services.SearchQuery{
		QueryString: "test",
		Page:        1,
		PageSize:    10,
	}

	results, err := updatedAccessor.Search(searchQuery)
	if err != nil {
		t.Fatalf("Failed to search after reindexing: %v", err)
	}

	if results.Total != 3 {
		t.Errorf("Expected 3 search results, got %d", results.Total)
	}
}

func TestEngine_AsyncReindexingWithNonExistentIndex(t *testing.T) {
	testDir := createTestDir(t)
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to remove test directory: %v", err)
		}
	}()

	engine := NewEngine(testDir)
	defer engine.jobManager.Stop()

	settings := config.IndexSettings{
		Name:             "non-existent",
		SearchableFields: []string{"title"},
	}

	_, err := engine.UpdateIndexSettingsWithAsyncReindex("non-existent", settings)
	if err == nil {
		t.Error("Expected error for non-existent index")
	}

	if err != nil && !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestEngine_ListJobsForIndex(t *testing.T) {
	testDir := createTestDir(t)
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to remove test directory: %v", err)
		}
	}()

	engine := NewEngine(testDir)
	defer engine.jobManager.Stop()

	// Create test indexes
	settings1 := config.IndexSettings{
		Name:             "index1",
		SearchableFields: []string{"title"},
	}
	settings2 := config.IndexSettings{
		Name:             "index2",
		SearchableFields: []string{"title"},
	}

	err := engine.CreateIndex(settings1)
	if err != nil {
		t.Fatalf("Failed to create index1: %v", err)
	}

	err = engine.CreateIndex(settings2)
	if err != nil {
		t.Fatalf("Failed to create index2: %v", err)
	}

	// Start async jobs for different indexes
	jobID1, err := engine.UpdateIndexSettingsWithAsyncReindex("index1", settings1)
	if err != nil {
		t.Fatalf("Failed to start job for index1: %v", err)
	}

	jobID2, err := engine.UpdateIndexSettingsWithAsyncReindex("index2", settings2)
	if err != nil {
		t.Fatalf("Failed to start job for index2: %v", err)
	}

	// List jobs for index1
	jobs1 := engine.ListJobs("index1", nil)
	if len(jobs1) != 1 {
		t.Errorf("Expected 1 job for index1, got %d", len(jobs1))
	}

	if len(jobs1) > 0 && jobs1[0].ID != jobID1 {
		t.Errorf("Expected job ID %s for index1, got %s", jobID1, jobs1[0].ID)
	}

	// List jobs for index2
	jobs2 := engine.ListJobs("index2", nil)
	if len(jobs2) != 1 {
		t.Errorf("Expected 1 job for index2, got %d", len(jobs2))
	}

	if len(jobs2) > 0 && jobs2[0].ID != jobID2 {
		t.Errorf("Expected job ID %s for index2, got %s", jobID2, jobs2[0].ID)
	}

	// List jobs for non-existent index
	jobs3 := engine.ListJobs("non-existent", nil)
	if len(jobs3) != 0 {
		t.Errorf("Expected 0 jobs for non-existent index, got %d", len(jobs3))
	}
}

// Helper functions
func createTestDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "engine_async_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Package testing provides utilities and helpers for testing the search engine.
package testing

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// TestDirRegistry tracks test directories for cleanup
type TestDirRegistry struct {
	mu   sync.Mutex
	dirs []string
}

var globalTestDirRegistry = &TestDirRegistry{}

// RegisterTestDir registers a test directory for cleanup
func (r *TestDirRegistry) RegisterTestDir(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirs = append(r.dirs, dir)
}

// CleanupAll removes all registered test directories
func (r *TestDirRegistry) CleanupAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, dir := range r.dirs {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("Warning: Failed to remove test directory %s: %v\n", dir, err)
		}
	}
	r.dirs = nil
}

// CreateTestEngine creates a new engine instance for testing with automatic cleanup
func CreateTestEngine(t *testing.T) *engine.Engine {
	testDir := fmt.Sprintf("./test_data_%d", time.Now().UnixNano())
	globalTestDirRegistry.RegisterTestDir(testDir)

	eng := engine.NewEngine(testDir)

	// Register cleanup function
	t.Cleanup(func() {
		// Note: Currently relying on test cleanup to handle job manager shutdown
		// The engine doesn't expose a public method to stop the job manager
		_ = eng // Prevent unused variable warning
	})

	return eng
}

// CreateTestIndex creates a test index with default settings
func CreateTestIndex(t *testing.T, eng *engine.Engine, indexName string) config.IndexSettings {
	settings := config.IndexSettings{
		Name:             indexName,
		SearchableFields: []string{"title", "content", "description"},
		FilterableFields: []string{"category", "year", "status", "popularity"},
		RankingCriteria: []config.RankingCriterion{
			{Field: "popularity", Order: "desc"},
		},
		MinWordSizeFor1Typo:  4,
		MinWordSizeFor2Typos: 7,
	}

	err := eng.CreateIndex(settings)
	require.NoError(t, err, "Failed to create test index")

	return settings
}

// AddTestDocuments adds a set of test documents to an index
func AddTestDocuments(t *testing.T, eng *engine.Engine, indexName string) []model.Document {
	indexAccessor, err := eng.GetIndex(indexName)
	require.NoError(t, err, "Failed to get index accessor")

	docs := []model.Document{
		{
			"documentID":  "doc1",
			"title":       "The Matrix",
			"content":     "A computer programmer discovers reality is a simulation",
			"description": "Sci-fi action movie about virtual reality",
			"category":    "movie",
			"year":        1999,
			"status":      "published",
			"popularity":  9.5,
		},
		{
			"documentID":  "doc2",
			"title":       "Inception",
			"content":     "A thief enters people's dreams to steal secrets",
			"description": "Mind-bending thriller about dream manipulation",
			"category":    "movie",
			"year":        2010,
			"status":      "published",
			"popularity":  9.2,
		},
		{
			"documentID":  "doc3",
			"title":       "Interstellar",
			"content":     "Astronauts travel through a wormhole to save humanity",
			"description": "Space epic about time dilation and love",
			"category":    "movie",
			"year":        2014,
			"status":      "published",
			"popularity":  8.8,
		},
	}

	err = indexAccessor.AddDocuments(docs)
	require.NoError(t, err, "Failed to add test documents")

	return docs
}

// JobPollingOptions configures job polling behavior
type JobPollingOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	LogProgress  bool
}

// DefaultJobPollingOptions returns sensible defaults for job polling
func DefaultJobPollingOptions() JobPollingOptions {
	return JobPollingOptions{
		Timeout:      10 * time.Second,
		PollInterval: 100 * time.Millisecond,
		LogProgress:  true,
	}
}

// WaitForJobCompletion polls a job until it completes or times out
func WaitForJobCompletion(t *testing.T, jobManager services.JobManager, jobID string, opts JobPollingOptions) *model.Job {
	timeout := time.After(opts.Timeout)
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	var job *model.Job
	var err error

	for {
		select {
		case <-timeout:
			t.Fatalf("Job %s did not complete within %v timeout", jobID, opts.Timeout)
		case <-ticker.C:
			job, err = jobManager.GetJob(jobID)
			require.NoError(t, err, "Failed to get job status")

			switch job.Status {
			case model.JobStatusCompleted:
				if opts.LogProgress {
					t.Logf("Job %s completed successfully in %v", jobID, job.CompletedAt.Sub(job.CreatedAt))
				}
				return job
			case model.JobStatusFailed:
				t.Fatalf("Job %s failed: %s", jobID, job.Error)
			case model.JobStatusRunning:
				if opts.LogProgress && job.Progress != nil {
					t.Logf("Job %s progress: %d/%d - %s",
						jobID,
						job.Progress.Current,
						job.Progress.Total,
						job.Progress.Message)
				}
			}
		}
	}
}

// AssertJobCompleted verifies that a job completed successfully
func AssertJobCompleted(t *testing.T, job *model.Job, expectedType model.JobType, expectedIndex string) {
	assert.Equal(t, model.JobStatusCompleted, job.Status, "Job should be completed")
	assert.Equal(t, expectedType, job.Type, "Job type should match")
	assert.Equal(t, expectedIndex, job.IndexName, "Job index name should match")
	assert.NotNil(t, job.CompletedAt, "Job should have completion timestamp")
	assert.Empty(t, job.Error, "Job should not have error")
}

// AsyncOperationTest represents a test case for async operations
type AsyncOperationTest struct {
	Name            string
	SetupFunc       func(t *testing.T, eng *engine.Engine) string                   // Returns index name
	OperationFunc   func(t *testing.T, eng *engine.Engine, indexName string) string // Returns job ID
	ValidateFunc    func(t *testing.T, eng *engine.Engine, indexName string, job *model.Job)
	ExpectedJobType model.JobType
}

// RunAsyncOperationTests runs a suite of async operation tests
func RunAsyncOperationTests(t *testing.T, tests []AsyncOperationTest) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			eng := CreateTestEngine(t)

			// Setup
			indexName := tt.SetupFunc(t, eng)

			// Execute operation
			jobID := tt.OperationFunc(t, eng, indexName)
			require.NotEmpty(t, jobID, "Job ID should not be empty")

			// Wait for completion
			job := WaitForJobCompletion(t, eng, jobID, DefaultJobPollingOptions())

			// Verify job completion
			AssertJobCompleted(t, job, tt.ExpectedJobType, indexName)

			// Custom validation
			if tt.ValidateFunc != nil {
				tt.ValidateFunc(t, eng, indexName, job)
			}
		})
	}
}

// SearchTestCase represents a test case for search operations
type SearchTestCase struct {
	Name          string
	Query         services.SearchQuery
	ExpectedCount int
	ExpectedFirst string // Expected first result document ID
	ValidateFunc  func(t *testing.T, results *services.SearchResult)
}

// RunSearchTests runs a suite of search tests against an index
func RunSearchTests(t *testing.T, indexAccessor services.IndexAccessor, tests []SearchTestCase) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			results, err := indexAccessor.Search(tt.Query)
			require.NoError(t, err, "Search should not fail")

			assert.Equal(t, tt.ExpectedCount, results.Total, "Result count should match")

			if tt.ExpectedFirst != "" && len(results.Hits) > 0 {
				firstDocID, exists := results.Hits[0].Document.GetDocumentID()
				require.True(t, exists, "First result should have document ID")
				assert.Equal(t, tt.ExpectedFirst, firstDocID, "First result should match expected")
			}

			if tt.ValidateFunc != nil {
				tt.ValidateFunc(t, &results)
			}
		})
	}
}

// CleanupTestDirs should be called in TestMain to clean up all test directories
func CleanupTestDirs() {
	globalTestDirRegistry.CleanupAll()
}

// TestMain ensures proper cleanup of test directories
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup test directories
	CleanupTestDirs()

	// Exit with the test result code
	os.Exit(code)
}

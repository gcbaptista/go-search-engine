package engine

import (
	"runtime"
	"sync"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/internal/jobs"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// Engine manages multiple search indexes.
// It implements the services.IndexManager interface.
type Engine struct {
	mu         sync.RWMutex
	indexes    map[string]*IndexInstance
	dataDir    string
	jobManager *jobs.Manager
}

// NewEngine creates a new search engine orchestrator.
func NewEngine(dataDir string) *Engine {
	// Calculate optimal worker count based on CPU cores
	// Use 2x CPU cores for I/O bound operations, with minimum of 4 and maximum of 16
	maxWorkers := runtime.NumCPU() * 2
	if maxWorkers < 4 {
		maxWorkers = 4
	}
	if maxWorkers > 16 {
		maxWorkers = 16
	}

	eng := &Engine{
		indexes:    make(map[string]*IndexInstance),
		dataDir:    dataDir,
		jobManager: jobs.NewManager(maxWorkers),
	}
	eng.jobManager.Start()
	eng.loadIndexesFromDisk()
	return eng
}

// GetIndex retrieves an index by its name.
func (e *Engine) GetIndex(name string) (services.IndexAccessor, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.indexes[name]
	if !exists {
		return nil, errors.NewIndexNotFoundError(name)
	}
	return instance, nil
}

// GetIndexSettings retrieves the settings for a specific index.
func (e *Engine) GetIndexSettings(name string) (config.IndexSettings, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, exists := e.indexes[name]
	if !exists {
		return config.IndexSettings{}, errors.NewIndexNotFoundError(name)
	}
	return *instance.settings, nil // Return a copy
}

// ListIndexes returns a list of all index names.
func (e *Engine) ListIndexes() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.indexes))
	for name := range e.indexes {
		names = append(names, name)
	}
	return names
}

// GetJob retrieves a job by its ID.
func (e *Engine) GetJob(jobID string) (*model.Job, error) {
	return e.jobManager.GetJob(jobID)
}

// ListJobs returns a list of jobs for a specific index, optionally filtered by status.
func (e *Engine) ListJobs(indexName string, status *model.JobStatus) []*model.Job {
	return e.jobManager.ListJobs(indexName, status)
}

// GetJobMetrics returns job performance metrics.
func (e *Engine) GetJobMetrics() jobs.JobMetricsData {
	return e.jobManager.GetMetrics()
}

// GetJobSuccessRate returns the success rate of jobs.
func (e *Engine) GetJobSuccessRate() float64 {
	metrics := e.jobManager.GetMetrics()
	totalCompleted := metrics.JobsCompleted + metrics.JobsFailed
	if totalCompleted == 0 {
		return 1.0 // No jobs yet, assume 100% success
	}
	return float64(metrics.JobsCompleted) / float64(totalCompleted)
}

// GetCurrentWorkload returns the current number of running jobs.
func (e *Engine) GetCurrentWorkload() int64 {
	return e.jobManager.GetCurrentWorkload()
}

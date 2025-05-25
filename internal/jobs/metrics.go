package jobs

import (
	"sync"
	"time"

	"github.com/gcbaptista/go-search-engine/model"
)

// JobMetricsData represents job metrics data without mutex (safe for copying)
type JobMetricsData struct {
	JobsCreated          int64                     `json:"jobs_created"`
	JobsCompleted        int64                     `json:"jobs_completed"`
	JobsFailed           int64                     `json:"jobs_failed"`
	TotalExecutionTime   time.Duration             `json:"total_execution_time_ns"`
	AverageExecutionTime time.Duration             `json:"average_execution_time_ns"`
	JobsByType           map[model.JobType]int64   `json:"jobs_by_type"`
	JobsByStatus         map[model.JobStatus]int64 `json:"jobs_by_status"`
	LastUpdated          time.Time                 `json:"last_updated"`
}

// JobMetrics tracks performance metrics for job operations
type JobMetrics struct {
	mu                   sync.RWMutex
	JobsCreated          int64                             `json:"jobs_created"`
	JobsCompleted        int64                             `json:"jobs_completed"`
	JobsFailed           int64                             `json:"jobs_failed"`
	TotalExecutionTime   time.Duration                     `json:"total_execution_time_ns"`
	AverageExecutionTime time.Duration                     `json:"average_execution_time_ns"`
	JobsByType           map[model.JobType]int64           `json:"jobs_by_type"`
	JobsByStatus         map[model.JobStatus]int64         `json:"jobs_by_status"`
	ExecutionTimesByType map[model.JobType][]time.Duration `json:"-"` // Not exported in JSON
	LastUpdated          time.Time                         `json:"last_updated"`
}

// NewJobMetrics creates a new metrics collector
func NewJobMetrics() *JobMetrics {
	return &JobMetrics{
		JobsByType:           make(map[model.JobType]int64),
		JobsByStatus:         make(map[model.JobStatus]int64),
		ExecutionTimesByType: make(map[model.JobType][]time.Duration),
		LastUpdated:          time.Now(),
	}
}

// RecordJobCreated increments job creation counter
func (m *JobMetrics) RecordJobCreated(jobType model.JobType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.JobsCreated++
	m.JobsByType[jobType]++
	m.JobsByStatus[model.JobStatusPending]++
	m.LastUpdated = time.Now()
}

// RecordJobStatusChange updates status counters
func (m *JobMetrics) RecordJobStatusChange(oldStatus, newStatus model.JobStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if oldStatus != "" {
		m.JobsByStatus[oldStatus]--
		if m.JobsByStatus[oldStatus] < 0 {
			m.JobsByStatus[oldStatus] = 0
		}
	}
	m.JobsByStatus[newStatus]++
	m.LastUpdated = time.Now()
}

// RecordJobCompleted records successful job completion
func (m *JobMetrics) RecordJobCompleted(jobType model.JobType, executionTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.JobsCompleted++
	m.TotalExecutionTime += executionTime

	// Update average execution time
	if m.JobsCompleted > 0 {
		m.AverageExecutionTime = m.TotalExecutionTime / time.Duration(m.JobsCompleted)
	}

	// Track execution times by type
	m.ExecutionTimesByType[jobType] = append(m.ExecutionTimesByType[jobType], executionTime)

	// Keep only last 100 execution times per type to prevent memory growth
	if len(m.ExecutionTimesByType[jobType]) > 100 {
		m.ExecutionTimesByType[jobType] = m.ExecutionTimesByType[jobType][1:]
	}

	m.LastUpdated = time.Now()
}

// RecordJobFailed records job failure
func (m *JobMetrics) RecordJobFailed(jobType model.JobType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.JobsFailed++
	m.LastUpdated = time.Now()
}

// GetMetrics returns a copy of current metrics without mutex (safe for copying)
func (m *JobMetrics) GetMetrics() JobMetricsData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create deep copy of maps
	jobsByType := make(map[model.JobType]int64)
	for k, v := range m.JobsByType {
		jobsByType[k] = v
	}

	jobsByStatus := make(map[model.JobStatus]int64)
	for k, v := range m.JobsByStatus {
		jobsByStatus[k] = v
	}

	return JobMetricsData{
		JobsCreated:          m.JobsCreated,
		JobsCompleted:        m.JobsCompleted,
		JobsFailed:           m.JobsFailed,
		TotalExecutionTime:   m.TotalExecutionTime,
		AverageExecutionTime: m.AverageExecutionTime,
		JobsByType:           jobsByType,
		JobsByStatus:         jobsByStatus,
		LastUpdated:          m.LastUpdated,
	}
}

// GetAverageExecutionTimeByType returns average execution time for a specific job type
func (m *JobMetrics) GetAverageExecutionTimeByType(jobType model.JobType) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	times := m.ExecutionTimesByType[jobType]
	if len(times) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range times {
		total += t
	}
	return total / time.Duration(len(times))
}

// GetSuccessRate returns the success rate (0.0 to 1.0)
func (m *JobMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCompleted := m.JobsCompleted + m.JobsFailed
	if totalCompleted == 0 {
		return 1.0 // No jobs yet, assume 100% success
	}
	return float64(m.JobsCompleted) / float64(totalCompleted)
}

// GetCurrentWorkload returns the number of currently active jobs
func (m *JobMetrics) GetCurrentWorkload() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.JobsByStatus[model.JobStatusPending] + m.JobsByStatus[model.JobStatusRunning]
}

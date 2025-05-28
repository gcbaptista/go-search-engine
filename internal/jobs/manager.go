package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/model"
)

// Manager handles background job execution and tracking
type Manager struct {
	mu       sync.RWMutex
	jobs     map[string]*model.Job
	workers  chan struct{} // Limits concurrent jobs
	stopChan chan struct{}
	wg       sync.WaitGroup
	metrics  *JobMetrics
}

// NewManager creates a new job manager with specified worker count
func NewManager(maxWorkers int) *Manager {
	return &Manager{
		jobs:     make(map[string]*model.Job),
		workers:  make(chan struct{}, maxWorkers),
		stopChan: make(chan struct{}),
		metrics:  NewJobMetrics(),
	}
}

// Start begins the job manager and starts background cleanup
func (m *Manager) Start() {
	log.Printf("Job manager started with %d max workers", cap(m.workers))

	// Start cleanup routine
	go m.cleanupRoutine()
}

// Stop gracefully shuts down the job manager
func (m *Manager) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	log.Printf("Job manager stopped")
}

// CreateJob creates a new job and returns its ID
func (m *Manager) CreateJob(jobType model.JobType, indexName string, metadata map[string]string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &model.Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Status:    model.JobStatusPending,
		IndexName: indexName,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}

	m.jobs[job.ID] = job
	m.metrics.RecordJobCreated(jobType)
	log.Printf("Created job %s (type: %s) for index '%s'", job.ID, job.Type, job.IndexName)
	return job.ID
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(jobID string) (*model.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, errors.NewJobNotFoundError(jobID)
	}

	// Return a copy to avoid race conditions
	jobCopy := *job
	if job.Progress != nil {
		progressCopy := *job.Progress
		jobCopy.Progress = &progressCopy
	}
	return &jobCopy, nil
}

// ListJobs returns all jobs for a specific index, optionally filtered by status
func (m *Manager) ListJobs(indexName string, status *model.JobStatus) []*model.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*model.Job
	for _, job := range m.jobs {
		if job.IndexName == indexName {
			if status == nil || job.Status == *status {
				// Return a copy
				jobCopy := *job
				if job.Progress != nil {
					progressCopy := *job.Progress
					jobCopy.Progress = &progressCopy
				}
				result = append(result, &jobCopy)
			}
		}
	}
	return result
}

// ExecuteJob runs a job function in a goroutine with proper tracking
func (m *Manager) ExecuteJob(jobID string, jobFunc func(ctx context.Context, job *model.Job) error) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return errors.NewJobNotFoundError(jobID)
	}

	if job.Status != model.JobStatusPending {
		m.mu.Unlock()
		return fmt.Errorf("job with ID '%s' is not in pending status (current: %s)", jobID, job.Status)
	}

	oldStatus := job.Status
	job.Status = model.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	m.metrics.RecordJobStatusChange(oldStatus, job.Status)
	m.mu.Unlock()

	// Acquire worker slot
	select {
	case m.workers <- struct{}{}:
		// Got worker slot
	case <-m.stopChan:
		m.updateJobStatus(jobID, model.JobStatusCancelled, "Job manager shutting down")
		return fmt.Errorf("job manager is shutting down")
	}

	m.wg.Add(1)
	go func() {
		defer func() {
			<-m.workers // Release worker slot
			m.wg.Done()
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		startTime := time.Now()

		// Execute the job function
		err := jobFunc(ctx, job)

		executionTime := time.Since(startTime)

		// Update job status and metrics based on result
		if err != nil {
			m.updateJobStatus(jobID, model.JobStatusFailed, err.Error())
			m.metrics.RecordJobFailed(job.Type)
			log.Printf("Job %s failed after %v: %v", jobID, executionTime, err)
		} else {
			m.updateJobStatus(jobID, model.JobStatusCompleted, "")
			m.metrics.RecordJobCompleted(job.Type, executionTime)
			log.Printf("Job %s completed successfully in %v", jobID, executionTime)
		}
	}()

	return nil
}

// UpdateJobProgress updates the progress of a running job
func (m *Manager) UpdateJobProgress(jobID string, current, total int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return
	}

	if job.Progress == nil {
		job.Progress = &model.JobProgress{}
	}

	job.Progress.Current = current
	job.Progress.Total = total
	job.Progress.Message = message
}

// updateJobStatus updates the status of a job (internal method)
func (m *Manager) updateJobStatus(jobID string, status model.JobStatus, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return
	}

	oldStatus := job.Status
	job.Status = status
	if errorMsg != "" {
		job.Error = errorMsg
	}

	if status == model.JobStatusCompleted || status == model.JobStatusFailed || status == model.JobStatusCancelled {
		now := time.Now()
		job.CompletedAt = &now
	}

	m.metrics.RecordJobStatusChange(oldStatus, status)
}

// cleanupRoutine runs periodic job cleanup
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour) // Cleanup every hour
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up completed jobs older than 24 hours
			m.CleanupOldJobs(24 * time.Hour)
		case <-m.stopChan:
			return
		}
	}
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (m *Manager) CleanupOldJobs(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for jobID, job := range m.jobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(m.jobs, jobID)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("Cleaned up %d old jobs", cleaned)
	}
}

// GetMetrics returns current job performance metrics
func (m *Manager) GetMetrics() JobMetricsData {
	return m.metrics.GetMetrics()
}

// GetJobSuccessRate returns the overall job success rate
func (m *Manager) GetJobSuccessRate() float64 {
	return m.metrics.GetSuccessRate()
}

// GetCurrentWorkload returns the number of currently active jobs
func (m *Manager) GetCurrentWorkload() int64 {
	return m.metrics.GetCurrentWorkload()
}

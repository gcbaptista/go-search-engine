package model

import (
	"time"
)

// JobStatus represents the status of a long-running job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelling JobStatus = "cancelling"
	JobStatusCancelled  JobStatus = "cancelled"
)

// JobType represents the type of job being executed
type JobType string

const (
	JobTypeReindex        JobType = "reindex"
	JobTypeUpdateSettings JobType = "update_settings"
	JobTypeCreateIndex    JobType = "create_index"
	JobTypeDeleteIndex    JobType = "delete_index"
	JobTypeAddDocuments   JobType = "add_documents"
	JobTypeDeleteAllDocs  JobType = "delete_all_docs"
	JobTypeDeleteDocument JobType = "delete_document"
	JobTypeRenameIndex    JobType = "rename_index"
)

// Job represents a long-running background operation
type Job struct {
	ID          string            `json:"id"`
	Type        JobType           `json:"type"`
	Status      JobStatus         `json:"status"`
	IndexName   string            `json:"index_name"`
	Progress    *JobProgress      `json:"progress,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// JobProgress tracks the progress of a job
type JobProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

// GetProgressPercentage returns the progress as a percentage (0-100)
func (jp *JobProgress) GetProgressPercentage() float64 {
	if jp.Total == 0 {
		return 0
	}
	return float64(jp.Current) / float64(jp.Total) * 100
}

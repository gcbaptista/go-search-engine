package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/model"
)

func TestJobManager_CreateJob(t *testing.T) {
	manager := NewManager(2)
	defer manager.Stop()

	jobID := manager.CreateJob(model.JobTypeReindex, "test-index", map[string]string{
		"operation": "test",
	})

	if jobID == "" {
		t.Error("Expected non-empty job ID")
	}

	job, err := manager.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get created job: %v", err)
	}

	if job.Type != model.JobTypeReindex {
		t.Errorf("Expected job type %s, got %s", model.JobTypeReindex, job.Type)
	}

	if job.Status != model.JobStatusPending {
		t.Errorf("Expected job status %s, got %s", model.JobStatusPending, job.Status)
	}

	if job.IndexName != "test-index" {
		t.Errorf("Expected index name 'test-index', got %s", job.IndexName)
	}
}

func TestJobManager_ExecuteJob(t *testing.T) {
	manager := NewManager(2)
	manager.Start()
	defer manager.Stop()

	jobID := manager.CreateJob(model.JobTypeReindex, "test-index", nil)

	// Execute a simple job that updates progress
	err := manager.ExecuteJob(jobID, func(ctx context.Context, job *model.Job) error {
		manager.UpdateJobProgress(jobID, 50, 100, "Halfway done")
		time.Sleep(10 * time.Millisecond) // Simulate work
		manager.UpdateJobProgress(jobID, 100, 100, "Completed")
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to execute job: %v", err)
	}

	// Wait a bit for job to complete
	time.Sleep(50 * time.Millisecond)

	job, err := manager.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job after execution: %v", err)
	}

	if job.Status != model.JobStatusCompleted {
		t.Errorf("Expected job status %s, got %s", model.JobStatusCompleted, job.Status)
	}

	if job.Progress == nil {
		t.Error("Expected job progress to be set")
	} else {
		if job.Progress.Current != 100 {
			t.Errorf("Expected progress current 100, got %d", job.Progress.Current)
		}
		if job.Progress.Total != 100 {
			t.Errorf("Expected progress total 100, got %d", job.Progress.Total)
		}
	}
}

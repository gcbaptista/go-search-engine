package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/internal/engine"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// GetJobHandler handles requests to get job status by ID
func (api *API) GetJobHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	if jobManager, ok := api.engine.(services.JobManager); ok {
		job, err := jobManager.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, job)
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job management not supported by this engine"})
	}
}

// ListJobsHandler handles requests to list jobs for an index
func (api *API) ListJobsHandler(c *gin.Context) {
	indexName := c.Param("indexName")
	statusParam := c.Query("status")

	var statusFilter *model.JobStatus
	if statusParam != "" {
		status := model.JobStatus(statusParam)
		statusFilter = &status
	}

	if jobManager, ok := api.engine.(services.JobManager); ok {
		jobs := jobManager.ListJobs(indexName, statusFilter)
		c.JSON(http.StatusOK, gin.H{
			"jobs":       jobs,
			"index_name": indexName,
			"total":      len(jobs),
		})
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job management not supported by this engine"})
	}
}

// GetJobMetricsHandler handles requests to get job performance metrics
func (api *API) GetJobMetricsHandler(c *gin.Context) {
	if engineWithMetrics, ok := api.engine.(*engine.Engine); ok {
		// Get metrics (already returns a copy without mutex)
		metrics := engineWithMetrics.GetJobMetrics()

		// Add computed metrics
		response := gin.H{
			"metrics":          metrics,
			"success_rate":     engineWithMetrics.GetJobSuccessRate(),
			"current_workload": engineWithMetrics.GetCurrentWorkload(),
		}

		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Job metrics not supported by this engine"})
	}
}

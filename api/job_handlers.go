package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gcbaptista/go-search-engine/internal/engine"
	internalErrors "github.com/gcbaptista/go-search-engine/internal/errors"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/services"
)

// GetJobHandler handles requests to get job status by ID
func (api *API) GetJobHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	if jobManager, ok := api.engine.(services.JobManager); ok {
		job, err := jobManager.GetJob(jobID)
		if err != nil {
			if errors.Is(err, internalErrors.ErrJobNotFound) {
				SendJobNotFoundError(c, jobID)
				return
			}
			SendInternalError(c, "get job", err)
			return
		}

		c.JSON(http.StatusOK, job)
	} else {
		SendError(c, http.StatusNotImplemented, ErrorCodeInternalError, "Job management not supported by this engine")
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
		SendError(c, http.StatusNotImplemented, ErrorCodeInternalError, "Job management not supported by this engine")
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
		SendError(c, http.StatusNotImplemented, ErrorCodeInternalError, "Job metrics not supported by this engine")
	}
}

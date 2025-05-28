package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetAnalyticsHandler handles the request to get analytics data
func (api *API) GetAnalyticsHandler(c *gin.Context) {
	dashboard, err := api.analytics.GetDashboardData()
	if err != nil {
		SendInternalError(c, "retrieve analytics data", err)
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// HealthCheckHandler provides a simple health check endpoint
func (api *API) HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "go-search-engine",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

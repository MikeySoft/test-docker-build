package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/mikeysoft/flotilla/internal/server/metrics"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

// MetricsHandler handles metrics-related API endpoints
type MetricsHandler struct {
	hub           *websocket.Hub
	metricsClient *metrics.Client
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(hub *websocket.Hub) *MetricsHandler {
	return &MetricsHandler{
		hub:           hub,
		metricsClient: hub.GetMetricsClient(),
	}
}

// GetHostMetrics returns metrics for a specific host
func (h *MetricsHandler) GetHostMetrics(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if metrics client is available
	if h.metricsClient == nil || !h.metricsClient.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Metrics storage not available",
		})
		return
	}

	// Parse query parameters
	startTime, endTime, interval := h.parseMetricsParams(c)

	// Query metrics from InfluxDB
	ctx := c.Request.Context()
	hostMetrics, err := h.metricsClient.QueryHostMetrics(ctx, hostID, startTime, endTime, interval)
	if err != nil {
		logrus.Errorf("Failed to query host metrics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve host metrics",
		})
		return
	}

	// Ensure metrics is always an array, not null
	if hostMetrics == nil {
		hostMetrics = []protocol.HostMetric{}
	}

	c.JSON(http.StatusOK, gin.H{
		"host_id": hostID,
		"metrics": hostMetrics,
	})
}

// GetContainerMetrics returns metrics for a specific container
func (h *MetricsHandler) GetContainerMetrics(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("container_id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if metrics client is available
	if h.metricsClient == nil || !h.metricsClient.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Metrics storage not available",
		})
		return
	}

	// Parse query parameters
	startTime, endTime, interval := h.parseMetricsParams(c)

	// Query metrics from InfluxDB
	ctx := c.Request.Context()
	containerMetrics, err := h.metricsClient.QueryContainerMetrics(ctx, hostID, containerID, startTime, endTime, interval)
	if err != nil {
		logrus.Errorf("Failed to query container metrics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve container metrics",
		})
		return
	}

	// Ensure metrics is always an array, not null
	if containerMetrics == nil {
		containerMetrics = []protocol.ContainerMetric{}
	}

	c.JSON(http.StatusOK, gin.H{
		"host_id":      hostID,
		"container_id": containerID,
		"metrics":      containerMetrics,
	})
}

// parseMetricsParams parses start, end, and interval parameters from query string
func (h *MetricsHandler) parseMetricsParams(c *gin.Context) (time.Time, time.Time, time.Duration) {
	// Default values
	now := time.Now()
	startTime := now.Add(-1 * time.Hour) // Default: last 1 hour
	endTime := now
	interval := 1 * time.Minute // Default: 1 minute interval

	// Parse start time
	if startStr := c.Query("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	// Parse end time
	if endStr := c.Query("end"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	// Parse interval
	if intervalStr := c.Query("interval"); intervalStr != "" {
		if parsed, err := time.ParseDuration(intervalStr); err == nil {
			interval = parsed
		}
	}

	return startTime, endTime, interval
}

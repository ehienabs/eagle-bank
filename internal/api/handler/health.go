package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthResponse represents health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// HealthCheck handles GET /health
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
	})
}

// ReadinessCheck handles GET /ready
// Returns 200 if the service is ready to accept traffic
func ReadinessCheck(c *gin.Context) {
	// In a real application, you would check:
	// - Database connectivity
	// - Kafka connectivity
	// - Any other dependencies

	c.JSON(http.StatusOK, HealthResponse{
		Status: "ready",
	})
}

// LivenessCheck handles GET /live
// Returns 200 if the service is alive
func LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "alive",
	})
}

// MetricsHandler returns a Gin handler for Prometheus metrics
func MetricsHandler(m *metrics.Metrics) gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

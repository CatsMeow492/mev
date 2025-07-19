package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
}

// HealthHandler handles health check requests
type HealthHandler struct {
	startTime time.Time
	version   string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
		version:   version,
	}
}

// Health returns the health status of the application
func (h *HealthHandler) Health(c *gin.Context) {
	uptime := time.Since(h.startTime)
	
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   h.version,
		Uptime:    uptime.String(),
	}
	
	c.JSON(http.StatusOK, response)
}

// Ready returns the readiness status of the application
func (h *HealthHandler) Ready(c *gin.Context) {
	// TODO: Add readiness checks for:
	// - Database connections
	// - RPC connections
	// - Fork manager status
	// - Queue status
	
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"timestamp": time.Now(),
	})
}
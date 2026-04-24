package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves service-level status endpoints.
type HealthHandler struct{}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Get handles GET /api/health.
func (h *HealthHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

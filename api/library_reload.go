package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Reload handles POST /api/library/reload.
func (h *LibraryHandler) Reload(c *gin.Context) {
	value, err := h.service.ReloadLibrary(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusAccepted, value)
}

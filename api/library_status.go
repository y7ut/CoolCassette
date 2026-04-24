package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Status handles GET /api/library/status.
func (h *LibraryHandler) Status(c *gin.Context) {
	value, err := h.service.GetLibraryStatus(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusOK, value)
}

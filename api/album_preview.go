package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Preview handles POST /api/albums/:id/preview.
func (h *AlbumHandler) Preview(c *gin.Context) {
	req := h.bindForceRequest(c)
	value, err := h.service.GeneratePreview(c.Request.Context(), h.albumID(c), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusOK, value)
}

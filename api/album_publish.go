package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Publish handles POST /api/albums/:id/publish.
func (h *AlbumHandler) Publish(c *gin.Context) {
	req := h.bindForceRequest(c)
	value, err := h.service.PublishAlbum(c.Request.Context(), h.albumID(c), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusOK, value)
}

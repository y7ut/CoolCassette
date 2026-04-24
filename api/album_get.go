package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Get handles GET /api/albums/:id.
func (h *AlbumHandler) Get(c *gin.Context) {
	value, err := h.service.GetAlbum(c.Request.Context(), h.albumID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusOK, value)
}

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// List handles GET /api/albums.
func (h *AlbumHandler) List(c *gin.Context) {
	value, err := h.service.ListAlbums(c.Request.Context(), h.bindListAlbumsRequest(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.JSON(http.StatusOK, value)
}

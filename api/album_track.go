package api

import "github.com/gin-gonic/gin"

// Asset handles GET /api/albums/:id/tracks/:name.
func (h *AlbumHandler) Track(c *gin.Context) {
	path, err := h.service.AlbumTrackPath(c.Request.Context(), h.albumID(c), h.assetName(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.File(path)
}

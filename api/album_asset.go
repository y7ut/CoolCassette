package api

import "github.com/gin-gonic/gin"

// Asset handles GET /api/albums/:id/assets/:name.
func (h *AlbumHandler) Asset(c *gin.Context) {
	path, err := h.service.AlbumAssetPath(c.Request.Context(), h.albumID(c), h.assetName(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.File(path)
}

// PublishedAsset handles GET /api/albums/:id/published/:name.
func (h *AlbumHandler) PublishedAsset(c *gin.Context) {
	path, err := h.service.PublishedAssetPath(c.Request.Context(), h.albumID(c), h.assetName(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeIndexHeaders(c, h.service.CurrentIndexSnapshot())
	c.File(path)
}

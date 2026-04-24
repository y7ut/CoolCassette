package api

import "github.com/gin-gonic/gin"

// AlbumHandler serves all album-related API endpoints.
type AlbumHandler struct {
	service AlbumService
}

// NewAlbumHandler creates an AlbumHandler backed by the given service.
func NewAlbumHandler(service AlbumService) *AlbumHandler {
	return &AlbumHandler{service: service}
}

// bindForceRequest decodes the shared force request payload.
func (h *AlbumHandler) bindForceRequest(c *gin.Context) ForceRequest {
	var req ForceRequest
	_ = c.ShouldBindJSON(&req)
	return req
}

// bindListAlbumsRequest decodes the indexed album list query and index headers.
func (h *AlbumHandler) bindListAlbumsRequest(c *gin.Context) ListAlbumsRequest {
	var query AlbumListQuery
	_ = c.ShouldBindQuery(&query)
	return ListAlbumsRequest{
		AlbumListQuery: query,
		IndexVersion:   c.GetHeader(HeaderIndexVersion),
		IndexHash:      c.GetHeader(HeaderIndexHash),
	}
}

// albumID returns the album identifier from the route path.
func (h *AlbumHandler) albumID(c *gin.Context) string {
	return c.Param("id")
}

// assetName returns the requested asset name from the route path.
func (h *AlbumHandler) assetName(c *gin.Context) string {
	return c.Param("name")
}

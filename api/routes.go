package api

import "github.com/gin-gonic/gin"

// RegisterRoutes wires all HTTP endpoints onto the provided Gin router.
func RegisterRoutes(router gin.IRouter, svc AlbumService) {
	healthHandler := NewHealthHandler()
	albumHandler := NewAlbumHandler(svc)
	libraryHandler := NewLibraryHandler(svc)

	router.GET("/api/health", healthHandler.Get)
	router.GET("/api/library/status", libraryHandler.Status)
	router.POST("/api/library/reload", libraryHandler.Reload)
	router.GET("/api/albums", albumHandler.List)
	router.GET("/api/albums/:id", albumHandler.Get)
	router.POST("/api/albums/:id/preview", albumHandler.Preview)
	router.POST("/api/albums/:id/publish", albumHandler.Publish)
	router.GET("/api/albums/:id/assets/:name", albumHandler.Asset)
	router.GET("/api/albums/:id/published/:name", albumHandler.PublishedAsset)
	router.GET("/api/albums/:id/tracks/:name", albumHandler.Track)
}

package api

// LibraryHandler serves library-wide index management endpoints.
type LibraryHandler struct {
	service AlbumService
}

// NewLibraryHandler creates a LibraryHandler backed by the given service.
func NewLibraryHandler(service AlbumService) *LibraryHandler {
	return &LibraryHandler{service: service}
}

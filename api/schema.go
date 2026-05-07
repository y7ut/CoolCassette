package api

import "context"

const (
	// HeaderIndexVersion is the response/request header carrying the active index version.
	HeaderIndexVersion = "X-CoolCassette-Index-Version"
	// HeaderIndexHash is the response/request header carrying the active index content hash.
	HeaderIndexHash = "X-CoolCassette-Index-Hash"
)

// IndexSnapshot identifies one active library index snapshot.
type IndexSnapshot struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

// AlbumListQuery defines the supported query string parameters for GET /api/albums.
type AlbumListQuery struct {
	Limit  int    `form:"limit"`
	SortBy string `form:"sort_by"`
	Order  string `form:"order"`
	Cursor string `form:"cursor"`
	Search string `form:"q"`
}

// ListAlbumsRequest is the full service request for a cursor-based album list query.
type ListAlbumsRequest struct {
	AlbumListQuery
	IndexVersion string
	IndexHash    string
}

// ForceRequest is the shared request payload for write actions that accept a force flag.
type ForceRequest struct {
	Force bool `json:"force"`
}

// ReloadRequest is the optional request payload for POST /api/library/reload.
type ReloadRequest struct {
	MusicDirs []string `json:"music_dirs"`
	WampyDir  string   `json:"wampy_dir"`
}

// AlbumService defines the album- and library-focused operations exposed to HTTP handlers.
type AlbumService interface {
	// CurrentIndexSnapshot returns the active index version/hash used for response headers.
	CurrentIndexSnapshot() IndexSnapshot
	// ListAlbums returns one cursor-based page of indexed album summaries.
	ListAlbums(context.Context, ListAlbumsRequest) (any, error)
	// GetAlbum returns lazily loaded detail data for a single indexed album.
	GetAlbum(context.Context, string) (any, error)
	// GeneratePreview creates or refreshes album-local preview images.
	GeneratePreview(context.Context, string, ForceRequest) (any, error)
	// PublishAlbum builds and deploys album assets into Wampy.
	PublishAlbum(context.Context, string, ForceRequest) (any, error)
	// GetLibraryStatus returns active index metadata and background scan state.
	GetLibraryStatus(context.Context) (any, error)
	// ReloadLibrary starts a background full re-scan of the music library.
	ReloadLibrary(context.Context, ReloadRequest) (any, error)
	// AlbumAssetPath resolves an on-disk path for album-local image assets.
	AlbumAssetPath(context.Context, string, string) (string, error)
	// PublishedAssetPath resolves an on-disk path for decoded deployed image assets.
	PublishedAssetPath(context.Context, string, string) (string, error)
	// AlbumTrackPath resolves an on-disk path for album-local audio files.
	AlbumTrackPath(context.Context, string, string) (string, error)
	// ClearCache removes all cache files except the index database.
	ClearCache(context.Context) (any, error)
}

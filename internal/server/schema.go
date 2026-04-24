package server

import (
	"time"

	"github.com/coolcassette/coolcassette/internal/wampy"
)

const (
	StatusBuilt        = "built"
	StatusPreviewReady = "preview_ready"
	StatusNotBuilt     = "not_built"
	reelAnimDelayMS    = 55
)

// AlbumSummary is the indexed summary returned by the paginated album list API.
type AlbumSummary struct {
	ID            string             `json:"id"`
	Dir           string             `json:"dir"`
	Name          string             `json:"name"`
	Slug          string             `json:"slug"`
	Artist        string             `json:"artist"`
	Album         string             `json:"album"`
	TrackCount    int                `json:"track_count"`
	Status        string             `json:"status"`
	HasCover      bool               `json:"has_cover"`
	Cassette      *wampy.CassetteRef `json:"cassette"`
	CassetteValid bool               `json:"cassette_ref_valid"`
	CoverURL      string             `json:"cover_url"`
	CreatedAt     time.Time          `json:"created_at"`
	ModifiedAt    time.Time          `json:"modified_at"`
}

// MusicFile describes one music file inside an album directory.
type MusicFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// AlbumDetail is the lazily loaded detail payload for a single album.
type AlbumDetail struct {
	AlbumSummary
	MusicFiles          []MusicFile       `json:"music_files"`
	PreviewTapePNGURL   string            `json:"preview_tape_png_url"`
	PreviewReelPNGURL   string            `json:"preview_reel_png_url"`
	TapeConfig          map[string]string `json:"tape_config"`
	ReelConfig          map[string]string `json:"reel_config"`
	ReelAtlasFrames     []string          `json:"reel_atlas_frames"`
	PublishedTapePNGURL string            `json:"published_tape_png_url"`
	PublishedReelPNGURL string            `json:"published_reel_png_url"`
	IndexVersion        string            `json:"index_version"`
	IndexHash           string            `json:"index_hash"`
}

// ListAlbumsResponse contains one cursor-based page of albums.
type ListAlbumsResponse struct {
	Items        []AlbumSummary `json:"items"`
	NextCursor   string         `json:"next_cursor,omitempty"`
	HasMore      bool           `json:"has_more"`
	IndexVersion string         `json:"index_version"`
	IndexHash    string         `json:"index_hash"`
}

// LibraryStatusResponse reports the active index and background scan progress.
type LibraryStatusResponse struct {
	IndexVersion   string     `json:"index_version"`
	IndexHash      string     `json:"index_hash"`
	AlbumCount     int        `json:"album_count"`
	Scanning       bool       `json:"scanning"`
	ScanID         string     `json:"scan_id,omitempty"`
	ScanStartedAt  time.Time  `json:"scan_started_at,omitempty"`
	ScanFinishedAt *time.Time `json:"scan_finished_at,omitempty"`
	ScanError      string     `json:"scan_error,omitempty"`
	ScannedAlbums  int        `json:"scanned_albums,omitempty"`
	TotalAlbums    int        `json:"total_albums,omitempty"`
}

// ReloadLibraryResponse acknowledges a background reindex request.
type ReloadLibraryResponse struct {
	Accepted     bool   `json:"accepted"`
	ScanID       string `json:"scan_id,omitempty"`
	IndexVersion string `json:"index_version"`
	IndexHash    string `json:"index_hash"`
	Scanning     bool   `json:"scanning"`
}

type albumRecord struct {
	AlbumSummary
	FirstAudioFile string
	ArtistSort     string
	AlbumSort      string
}

type indexMetadata struct {
	Version    string
	Hash       string
	AlbumCount int
	BuiltAt    time.Time
	DBPath     string
}

type scanState struct {
	Active        bool
	ScanID        string
	StartedAt     time.Time
	FinishedAt    *time.Time
	Error         string
	ScannedAlbums int
	TotalAlbums   int
}

type listCursor struct {
	SortBy       string `json:"sort_by"`
	Order        string `json:"order"`
	LastValue    string `json:"last_value"`
	LastID       string `json:"last_id"`
	IndexVersion string `json:"index_version"`
	IndexHash    string `json:"index_hash"`
}

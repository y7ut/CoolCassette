package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coolcassette/coolcassette/api"
	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/wampy"
)

// ListAlbums returns one cursor-based page of indexed album summaries.
func (a *App) ListAlbums(ctx context.Context, request api.ListAlbumsRequest) (any, error) {
	if err := a.validateIndexSnapshot(request.IndexVersion, request.IndexHash); err != nil {
		return nil, err
	}

	db, meta, err := a.activeIndex()
	if err != nil {
		return nil, err
	}
	sortBy := normalizeSortBy(request.SortBy)
	order := orderDirection(request.Order)
	limit := request.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := make([]any, 0, 4)
	query := `SELECT id, dir, name, slug, artist, album, track_count, status, cassette_tape, cassette_reel,
		cassette_ref_valid, has_cover, created_at, modified_at FROM albums`
	if request.Cursor != "" {
		cursor, err := decodeCursor(request.Cursor)
		if err != nil {
			return nil, err
		}
		if err := a.validateIndexSnapshot(cursor.IndexVersion, cursor.IndexHash); err != nil {
			return nil, err
		}
		if cursor.SortBy != sortBy || orderDirection(cursor.Order) != order {
			return nil, &indexConflictError{current: a.CurrentIndexSnapshot()}
		}
		column := sortColumn(sortBy)
		comparator := ">"
		if order == "DESC" {
			comparator = "<"
		}
		query += " WHERE (" + column + " " + comparator + " ? OR (" + column + " = ? AND id " + comparator + " ?))"
		cursorValue, err := queryCursorValue(sortBy, cursor.LastValue)
		if err != nil {
			return nil, err
		}
		args = append(args, cursorValue, cursorValue, cursor.LastID)
	}
	query += " ORDER BY " + sortColumn(sortBy) + " " + order + ", id " + order + " LIMIT ?"
	args = append(args, limit+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AlbumSummary, 0, limit+1)
	for rows.Next() {
		summary, err := scanAlbumSummaryRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	response := &ListAlbumsResponse{
		Items:        items,
		HasMore:      hasMore,
		IndexVersion: meta.Version,
		IndexHash:    meta.Hash,
	}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		cursor, err := encodeCursor(listCursor{
			SortBy:       sortBy,
			Order:        strings.ToLower(request.Order),
			LastValue:    cursorValueString(sortBy, last),
			LastID:       last.ID,
			IndexVersion: meta.Version,
			IndexHash:    meta.Hash,
		})
		if err != nil {
			return nil, err
		}
		response.NextCursor = cursor
	}
	return response, nil
}

// GetAlbum returns lazily loaded detail data for a single indexed album.
func (a *App) GetAlbum(ctx context.Context, id string) (any, error) {
	record, meta, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return nil, err
	}

	files, err := listMusicFiles(record.Dir)
	if err != nil {
		return nil, err
	}

	detail := &AlbumDetail{
		AlbumSummary: record.AlbumSummary,
		MusicFiles:   files,
		IndexVersion: meta.Version,
		IndexHash:    meta.Hash,
	}

	if record.Status == StatusPreviewReady {
		detail.PreviewTapePNGURL = fmt.Sprintf("/api/albums/%s/preview/tape.png", id)
		detail.PreviewReelPNGURL = fmt.Sprintf("/api/albums/%s/preview/reel.png", id)
	}

	if record.Cassette != nil {
		tapeDir := wampy.TapeDir(a.cfg.WampyDir, record.Cassette.Tape)
		if cfg, err := wampy.ReadKeyValueFile(filepath.Join(tapeDir, "config.txt")); err == nil {
			detail.TapeConfig = cfg
		}
		reelDir := wampy.ReelDir(a.cfg.WampyDir, record.Cassette.Reel)
		if cfg, err := wampy.ReadKeyValueFile(filepath.Join(reelDir, "config.txt")); err == nil {
			detail.ReelConfig = cfg
		}
		if frames, err := wampy.ReadAtlasFrames(filepath.Join(reelDir, "atlas.txt")); err == nil {
			detail.ReelAtlasFrames = frames
		}
		if record.CassetteValid {
			detail.PublishedTapePNGURL = fmt.Sprintf("/api/albums/%s/published/tape.png", id)
			detail.PublishedReelPNGURL = fmt.Sprintf("/api/albums/%s/published/reel.png", id)
		}
	}
	return detail, nil
}

func listMusicFiles(dir string) ([]MusicFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]MusicFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name()[0] == '.' {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp3", ".flac", ".wav", ".m4a", ".m4b", ".aac", ".mp4":
			fullPath := filepath.Join(dir, entry.Name())
			tm := audio.ReadTrackMeta(fullPath)
			files = append(files, MusicFile{
				Name:        entry.Name(),
				Path:        fullPath,
				Artist:      tm.Artist,
				Title:       tm.Title,
				Album:       tm.Album,
				TrackNumber: tm.TrackNumber,
			})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].TrackNumber != files[j].TrackNumber {
			if files[j].TrackNumber == 0 {
				return true
			}
			if files[i].TrackNumber == 0 {
				return false
			}
			return files[i].TrackNumber < files[j].TrackNumber
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

func (a *App) coverCacheForAlbum(record albumRecord) (string, error) {
	cacheDir := filepath.Join(a.cacheDir, record.ID)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(cacheDir, ".cover.ccimg")
	firstInfo, err := os.Stat(record.FirstAudioFile)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dst); err == nil && !info.ModTime().Before(firstInfo.ModTime()) {
		return dst, nil
	}
	cover, err := audio.ExtractCover(record.FirstAudioFile)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, cover.Data, 0644); err != nil {
		return "", err
	}
	return dst, nil
}

func scanAlbumSummaryRow(rows interface{ Scan(dest ...any) error }) (AlbumSummary, error) {
	var summary AlbumSummary
	var cassetteTapeVal, cassetteReelVal string
	var cassetteValid, hasCover int
	var createdAt, modifiedAt int64
	if err := rows.Scan(
		&summary.ID,
		&summary.Dir,
		&summary.Name,
		&summary.Slug,
		&summary.Artist,
		&summary.Album,
		&summary.TrackCount,
		&summary.Status,
		&cassetteTapeVal,
		&cassetteReelVal,
		&cassetteValid,
		&hasCover,
		&createdAt,
		&modifiedAt,
	); err != nil {
		return AlbumSummary{}, err
	}
	summary.CassetteValid = intToBool(cassetteValid)
	summary.HasCover = intToBool(hasCover)
	summary.CreatedAt = time.Unix(createdAt, 0).UTC()
	summary.ModifiedAt = time.Unix(modifiedAt, 0).UTC()
	summary.CoverURL = fmt.Sprintf("/api/albums/%s/assets/cover.png", summary.ID)
	if cassetteTapeVal != "" || cassetteReelVal != "" {
		summary.Cassette = &wampy.CassetteRef{Tape: cassetteTapeVal, Reel: cassetteReelVal}
	}
	return summary, nil
}

func queryCursorValue(sortBy, raw string) (any, error) {
	switch sortBy {
	case "created_at", "modified_at":
		return strconv.ParseInt(raw, 10, 64)
	default:
		return raw, nil
	}
}

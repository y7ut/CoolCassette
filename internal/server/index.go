package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/coolcassette/coolcassette/api"
	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/scanner"
	"github.com/coolcassette/coolcassette/internal/wampy"
)

// CurrentIndexSnapshot returns the active index version and content hash.
func (a *App) CurrentIndexSnapshot() api.IndexSnapshot {
	a.indexMu.RLock()
	defer a.indexMu.RUnlock()
	return api.IndexSnapshot{
		Version: a.activeMeta.Version,
		Hash:    a.activeMeta.Hash,
	}
}

func (a *App) loadInitialIndex() error {
	a.inheritFromState()

	if !a.cfg.Force {
		meta, ok := a.loadIndexState()
		if ok && a.configMatchesState(meta) {
			dbPath := filepath.Join(a.indexDir, meta.DBFile)
			db, err := sql.Open("sqlite3", dbPath)
			if err == nil {
				if err := db.Ping(); err == nil {
					return a.swapActiveIndex(db, meta)
				}
				_ = db.Close()
			}
		}
	}
	db, meta, err := a.buildIndexDatabase(func(update scanState) {
		a.setScanState(update)
	})
	if err != nil {
		return err
	}
	if err := a.saveIndexState(meta); err != nil {
		_ = db.Close()
		return fmt.Errorf("save index state: %w", err)
	}
	return a.swapActiveIndex(db, meta)
}

func (a *App) inheritFromState() {
	meta, ok := a.loadIndexState()
	if !ok {
		return
	}
	if len(a.cfg.MusicDirs) == 0 && len(meta.MusicDirs) > 0 {
		a.cfg.MusicDirs = meta.MusicDirs
	}
	if a.cfg.WampyDir == "" && meta.WampyDir != "" {
		a.cfg.WampyDir = meta.WampyDir
	}
}

func (a *App) buildIndexDatabase(progress func(scanState)) (*sql.DB, indexMetadata, error) {
	version := time.Now().UTC().Format("20060102T150405.000000000Z07:00")
	scanID := version
	startedAt := time.Now().UTC()
	progress(scanState{Active: true, ScanID: scanID, StartedAt: startedAt})

	var albums []scanner.Album
	seen := make(map[string]bool)
	for _, dir := range a.cfg.MusicDirs {
		result, err := scanner.Scan(dir, true)
		if err != nil {
			finished := time.Now().UTC()
			progress(scanState{Active: false, ScanID: scanID, StartedAt: startedAt, FinishedAt: &finished, Error: err.Error()})
			return nil, indexMetadata{}, err
		}
		for _, a := range result {
			if !seen[a.Dir] {
				seen[a.Dir] = true
				albums = append(albums, a)
			}
		}
	}
	sort.Slice(albums, func(i, j int) bool { return albums[i].Dir < albums[j].Dir })

	dbPath := filepath.Join(a.indexDir, fmt.Sprintf("%s.db", sanitizeFileComponent(version)))
	_ = os.Remove(dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, indexMetadata{}, err
	}
	if err := initializeSchema(db); err != nil {
		_ = db.Close()
		return nil, indexMetadata{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO albums (
			id, dir, name, slug, artist, artist_sort, album, album_sort,
			track_count, status, cassette_tape, cassette_reel, cassette_ref_valid,
			has_cover, created_at, modified_at, first_audio_file
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	defer stmt.Close()

	hasher := sha256.New()
	progress(scanState{Active: true, ScanID: scanID, StartedAt: startedAt, TotalAlbums: len(albums)})
	for i, album := range albums {
		record, err := a.scanAlbumRecord(album)
		if err != nil {
			_ = tx.Rollback()
			_ = db.Close()
			return nil, indexMetadata{}, err
		}
		if _, err := stmt.Exec(
			record.ID,
			record.Dir,
			record.Name,
			record.Slug,
			record.Artist,
			record.ArtistSort,
			record.Album,
			record.AlbumSort,
			record.TrackCount,
			record.Status,
			cassetteTape(record.Cassette),
			cassetteReel(record.Cassette),
			boolToInt(record.CassetteValid),
			boolToInt(record.HasCover),
			record.CreatedAt.Unix(),
			record.ModifiedAt.Unix(),
			record.FirstAudioFile,
		); err != nil {
			_ = tx.Rollback()
			_ = db.Close()
			return nil, indexMetadata{}, err
		}
		fmt.Fprintf(hasher, "%s|%s|%s|%s|%d|%s|%t|%s|%s|%d|%d\n",
			record.ID, record.Dir, record.Artist, record.Album, record.TrackCount,
			record.Status, record.HasCover, cassetteTape(record.Cassette), cassetteReel(record.Cassette),
			record.CreatedAt.Unix(), record.ModifiedAt.Unix(),
		)
		progress(scanState{
			Active:        true,
			ScanID:        scanID,
			StartedAt:     startedAt,
			ScannedAlbums: i + 1,
			TotalAlbums:   len(albums),
		})
	}

	contentHash := hex.EncodeToString(hasher.Sum(nil))
	if err := upsertMeta(tx, "index_version", version); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	if err := upsertMeta(tx, "index_hash", contentHash); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	if err := upsertMeta(tx, "album_count", strconv.Itoa(len(albums))); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	if err := upsertMeta(tx, "built_at", startedAt.Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, indexMetadata{}, err
	}
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return nil, indexMetadata{}, err
	}

	finished := time.Now().UTC()
	progress(scanState{
		Active:        false,
		ScanID:        scanID,
		StartedAt:     startedAt,
		FinishedAt:    &finished,
		ScannedAlbums: len(albums),
		TotalAlbums:   len(albums),
	})
	return db, indexMetadata{
		Version:    version,
		Hash:       contentHash,
		AlbumCount: len(albums),
		BuiltAt:    startedAt,
		DBFile:     filepath.Base(dbPath),
		MusicDirs:  a.cfg.MusicDirs,
		WampyDir:   a.cfg.WampyDir,
	}, nil
}

func (a *App) scanAlbumRecord(album scanner.Album) (albumRecord, error) {
	artist := "Unknown Artist"
	albumName := filepath.Base(album.Dir)
	meta := audio.ReadAlbumMeta(album.FirstAudioFile)
	if strings.TrimSpace(meta.Artist) != "" {
		artist = strings.TrimSpace(meta.Artist)
	}
	if strings.TrimSpace(meta.Album) != "" {
		albumName = strings.TrimSpace(meta.Album)
	}

	files, err := listMusicFiles(album.Dir)
	if err != nil {
		return albumRecord{}, err
	}
	hasCover := false
	if _, err := audio.ExtractCover(album.FirstAudioFile); err == nil {
		hasCover = true
	}

	dirInfo, err := os.Stat(album.Dir)
	if err != nil {
		return albumRecord{}, err
	}
	createdAt := fileCreatedAt(dirInfo)
	modifiedAt := dirInfo.ModTime().UTC()

	musicDir := a.findMusicDir(album.Dir)
	record := albumRecord{
		AlbumSummary: AlbumSummary{
			ID:         albumID(musicDir, album.Dir),
			Dir:        album.Dir,
			Name:       album.Name,
			Slug:       album.Slug,
			Artist:     artist,
			Album:      albumName,
			TrackCount: len(files),
			HasCover:   hasCover,
			CoverURL:   fmt.Sprintf("/api/albums/%s/assets/cover.png", albumID(musicDir, album.Dir)),
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
		},
		FirstAudioFile: album.FirstAudioFile,
		ArtistSort:     strings.ToLower(artist),
		AlbumSort:      strings.ToLower(albumName),
	}

	ref, err := wampy.ReadCassette(album.Dir)
	switch {
	case err == nil && a.cfg.WampyDir != "":
		validation := wampy.ValidateCassetteRef(a.cfg.WampyDir, ref)
		record.Cassette = &ref
		record.CassetteValid = validation.Built()
		switch {
		case validation.Built():
			record.Status = StatusBuilt
		case fileExists(filepath.Join(album.Dir, "tape.png")):
			record.Status = StatusPreviewReady
		default:
			record.Status = StatusNotBuilt
		}
	default:
		if fileExists(filepath.Join(album.Dir, "tape.png")) {
			record.Status = StatusPreviewReady
		} else {
			record.Status = StatusNotBuilt
		}
	}
	return record, nil
}

func initializeSchema(db *sql.DB) error {
	schema := `
	PRAGMA journal_mode=WAL;
	CREATE TABLE IF NOT EXISTS albums (
		id TEXT PRIMARY KEY,
		dir TEXT NOT NULL,
		name TEXT NOT NULL,
		slug TEXT NOT NULL,
		artist TEXT NOT NULL,
		artist_sort TEXT NOT NULL,
		album TEXT NOT NULL,
		album_sort TEXT NOT NULL,
		track_count INTEGER NOT NULL,
		status TEXT NOT NULL,
		cassette_tape TEXT NOT NULL,
		cassette_reel TEXT NOT NULL,
		cassette_ref_valid INTEGER NOT NULL,
		has_cover INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		modified_at INTEGER NOT NULL,
		first_audio_file TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums (artist_sort, id);
	CREATE INDEX IF NOT EXISTS idx_albums_album ON albums (album_sort, id);
	CREATE INDEX IF NOT EXISTS idx_albums_created ON albums (created_at, id);
	CREATE INDEX IF NOT EXISTS idx_albums_modified ON albums (modified_at, id);
	CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`
	_, err := db.Exec(schema)
	return err
}

func upsertMeta(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (a *App) swapActiveIndex(db *sql.DB, meta indexMetadata) error {
	a.indexMu.Lock()
	defer a.indexMu.Unlock()
	oldDB := a.activeDB
	a.activeDB = db
	a.activeMeta = meta
	if oldDB != nil {
		_, _ = oldDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		_ = oldDB.Close()
	}
	a.cleanupOldDBs(meta.DBFile)
	return nil
}

func (a *App) activeIndex() (*sql.DB, indexMetadata, error) {
	a.indexMu.RLock()
	defer a.indexMu.RUnlock()
	if a.activeDB == nil {
		return nil, indexMetadata{}, fmt.Errorf("index not initialized")
	}
	return a.activeDB, a.activeMeta, nil
}

func encodeCursor(cursor listCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCursor(raw string) (listCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return listCursor{}, err
	}
	var cursor listCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return listCursor{}, err
	}
	return cursor, nil
}

func cassetteTape(ref *wampy.CassetteRef) string {
	if ref == nil {
		return ""
	}
	return ref.Tape
}

func cassetteReel(ref *wampy.CassetteRef) string {
	if ref == nil {
		return ""
	}
	return ref.Reel
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intToBool(value int) bool {
	return value != 0
}

type indexConflictError struct {
	current api.IndexSnapshot
}

func (e *indexConflictError) Error() string {
	return "index content changed"
}

func (e *indexConflictError) ConflictMetadata() api.IndexSnapshot {
	return e.current
}

func (a *App) validateIndexSnapshot(version, hash string) error {
	current := a.CurrentIndexSnapshot()
	if version == "" {
		return nil
	}
	if version == current.Version {
		return nil
	}
	if hash != "" && hash == current.Hash {
		return nil
	}
	return &indexConflictError{current: current}
}

func (a *App) setScanState(state scanState) {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	a.scanState = state
}

func (a *App) currentScanState() scanState {
	a.scanMu.RLock()
	defer a.scanMu.RUnlock()
	return a.scanState
}

func orderDirection(order string) string {
	if strings.EqualFold(order, "desc") {
		return "DESC"
	}
	return "ASC"
}

func normalizeSortBy(sortBy string) string {
	switch strings.ToLower(sortBy) {
	case "artist":
		return "artist"
	case "created_at":
		return "created_at"
	case "modified_at":
		return "modified_at"
	default:
		return "album"
	}
}

func sortColumn(sortBy string) string {
	switch sortBy {
	case "artist":
		return "artist_sort"
	case "created_at":
		return "created_at"
	case "modified_at":
		return "modified_at"
	default:
		return "album_sort"
	}
}

func cursorValueString(sortBy string, summary AlbumSummary) string {
	switch sortBy {
	case "artist":
		return strings.ToLower(summary.Artist)
	case "created_at":
		return strconv.FormatInt(summary.CreatedAt.Unix(), 10)
	case "modified_at":
		return strconv.FormatInt(summary.ModifiedAt.Unix(), 10)
	default:
		return strings.ToLower(summary.Album)
	}
}

func (a *App) getAlbumRecordByID(ctx context.Context, id string) (albumRecord, indexMetadata, error) {
	db, meta, err := a.activeIndex()
	if err != nil {
		return albumRecord{}, indexMetadata{}, err
	}
	query := `SELECT id, dir, name, slug, artist, artist_sort, album, album_sort, track_count, status,
		cassette_tape, cassette_reel, cassette_ref_valid, has_cover, created_at, modified_at, first_audio_file
		FROM albums WHERE id = ?`
	var record albumRecord
	var cassetteTapeVal, cassetteReelVal string
	var cassetteValid, hasCover int
	var createdAt, modifiedAt int64
	err = db.QueryRowContext(ctx, query, id).Scan(
		&record.ID, &record.Dir, &record.Name, &record.Slug, &record.Artist, &record.ArtistSort,
		&record.Album, &record.AlbumSort, &record.TrackCount, &record.Status,
		&cassetteTapeVal, &cassetteReelVal, &cassetteValid, &hasCover, &createdAt, &modifiedAt, &record.FirstAudioFile,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return albumRecord{}, indexMetadata{}, os.ErrNotExist
		}
		return albumRecord{}, indexMetadata{}, err
	}
	record.CassetteValid = intToBool(cassetteValid)
	record.HasCover = intToBool(hasCover)
	record.CreatedAt = time.Unix(createdAt, 0).UTC()
	record.ModifiedAt = time.Unix(modifiedAt, 0).UTC()
	record.CoverURL = fmt.Sprintf("/api/albums/%s/assets/cover.png", record.ID)
	if cassetteTapeVal != "" || cassetteReelVal != "" {
		record.Cassette = &wampy.CassetteRef{Tape: cassetteTapeVal, Reel: cassetteReelVal}
	}
	return record, meta, nil
}

func (a *App) refreshAlbumInActiveIndex(ctx context.Context, id string) error {
	current, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return err
	}

	updated, err := a.scanAlbumRecord(scanner.Album{
		Dir:            current.Dir,
		Name:           current.Name,
		Slug:           current.Slug,
		FirstAudioFile: current.FirstAudioFile,
	})
	if err != nil {
		return err
	}

	a.indexMu.Lock()
	defer a.indexMu.Unlock()

	if a.activeDB == nil {
		return fmt.Errorf("index not initialized")
	}

	tx, err := a.activeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE albums
		SET dir = ?, name = ?, slug = ?, artist = ?, artist_sort = ?, album = ?, album_sort = ?,
			track_count = ?, status = ?, cassette_tape = ?, cassette_reel = ?, cassette_ref_valid = ?,
			has_cover = ?, created_at = ?, modified_at = ?, first_audio_file = ?
		WHERE id = ?`,
		updated.Dir,
		updated.Name,
		updated.Slug,
		updated.Artist,
		updated.ArtistSort,
		updated.Album,
		updated.AlbumSort,
		updated.TrackCount,
		updated.Status,
		cassetteTape(updated.Cassette),
		cassetteReel(updated.Cassette),
		boolToInt(updated.CassetteValid),
		boolToInt(updated.HasCover),
		updated.CreatedAt.Unix(),
		updated.ModifiedAt.Unix(),
		updated.FirstAudioFile,
		updated.ID,
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	newHash, albumCount, err := computeIndexHash(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	newVersion := a.activeMeta.Version
	newBuiltAt := a.activeMeta.BuiltAt
	if newHash != a.activeMeta.Hash {
		now := time.Now().UTC()
		newVersion = now.Format("20060102T150405.000000000Z07:00")
		newBuiltAt = now
	}

	if err := upsertMeta(tx, "index_version", newVersion); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := upsertMeta(tx, "index_hash", newHash); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := upsertMeta(tx, "album_count", strconv.Itoa(albumCount)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := upsertMeta(tx, "built_at", newBuiltAt.Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	a.activeMeta.Version = newVersion
	a.activeMeta.Hash = newHash
	a.activeMeta.AlbumCount = albumCount
	a.activeMeta.BuiltAt = newBuiltAt
	return nil
}

type rowsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func computeIndexHash(ctx context.Context, q rowsQueryer) (string, int, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, dir, artist, album, track_count, status, has_cover, cassette_tape, cassette_reel, created_at, modified_at
		FROM albums
		ORDER BY id ASC`)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	hasher := sha256.New()
	count := 0
	for rows.Next() {
		var (
			id, dir, artist, album, status, tape, reel string
			trackCount, hasCover                        int
			createdAt, modifiedAt                       int64
		)
		if err := rows.Scan(&id, &dir, &artist, &album, &trackCount, &status, &hasCover, &tape, &reel, &createdAt, &modifiedAt); err != nil {
			return "", 0, err
		}
		fmt.Fprintf(hasher, "%s|%s|%s|%s|%d|%s|%t|%s|%s|%d|%d\n",
			id, dir, artist, album, trackCount, status, intToBool(hasCover), tape, reel, createdAt, modifiedAt,
		)
		count++
	}
	if err := rows.Err(); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), count, nil
}

const indexStateFile = "state.json"

func (a *App) loadIndexState() (indexMetadata, bool) {
	data, err := os.ReadFile(filepath.Join(a.indexDir, indexStateFile))
	if err != nil {
		return indexMetadata{}, false
	}
	var meta indexMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return indexMetadata{}, false
	}
	return meta, true
}

func (a *App) saveIndexState(meta indexMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.indexDir, indexStateFile), data, 0644)
}

func (a *App) configMatchesState(meta indexMetadata) bool {
	if normalizePath(meta.WampyDir) != normalizePath(a.cfg.WampyDir) {
		return false
	}
	return normalizedPathsEqual(meta.MusicDirs, a.cfg.MusicDirs)
}

func normalizedPathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	an := make([]string, len(a))
	bn := make([]string, len(b))
	for i, p := range a {
		an[i] = normalizePath(p)
	}
	for i, p := range b {
		bn[i] = normalizePath(p)
	}
	sort.Strings(an)
	sort.Strings(bn)
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}

func normalizePath(p string) string {
	return strings.TrimRight(filepath.Clean(p), string(filepath.Separator))
}

func (a *App) findMusicDir(albumDir string) string {
	for _, dir := range a.cfg.MusicDirs {
		rel, err := filepath.Rel(dir, albumDir)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return dir
		}
	}
	return ""
}

func (a *App) cleanupOldDBs(keepFile string) {
	entries, err := os.ReadDir(a.indexDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == keepFile || name == indexStateFile {
			continue
		}
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm") {
			base := name
			if idx := strings.Index(base, ".db"); idx >= 0 {
				base = base[:idx+3]
			}
			if base == keepFile {
				continue
			}
			_ = os.Remove(filepath.Join(a.indexDir, name))
		}
	}
}

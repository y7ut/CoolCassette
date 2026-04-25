package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coolcassette/coolcassette/api"
)

// GetLibraryStatus returns the active index metadata and current background scan state.
func (a *App) GetLibraryStatus(context.Context) (any, error) {
	snapshot := a.CurrentIndexSnapshot()
	a.indexMu.RLock()
	albumCount := a.activeMeta.AlbumCount
	a.indexMu.RUnlock()
	scan := a.currentScanState()
	return &LibraryStatusResponse{
		IndexVersion:   snapshot.Version,
		IndexHash:      snapshot.Hash,
		AlbumCount:     albumCount,
		MusicDirs:      a.cfg.MusicDirs,
		WampyDir:       a.cfg.WampyDir,
		Scanning:       scan.Active,
		ScanID:         scan.ScanID,
		ScanStartedAt:  scan.StartedAt,
		ScanFinishedAt: scan.FinishedAt,
		ScanError:      scan.Error,
		ScannedAlbums:  scan.ScannedAlbums,
		TotalAlbums:    scan.TotalAlbums,
	}, nil
}

// ReloadLibrary starts an asynchronous full re-scan and swaps in a new index when it completes.
func (a *App) ReloadLibrary(_ context.Context, req api.ReloadRequest) (any, error) {
	scan := a.currentScanState()
	if scan.Active {
		snapshot := a.CurrentIndexSnapshot()
		return &ReloadLibraryResponse{
			Accepted:     false,
			ScanID:       scan.ScanID,
			IndexVersion: snapshot.Version,
			IndexHash:    snapshot.Hash,
			Scanning:     true,
		}, nil
	}

	if len(req.MusicDirs) > 0 {
		a.cfg.MusicDirs = req.MusicDirs
	}
	if req.WampyDir != "" {
		a.cfg.WampyDir = req.WampyDir
	}

	scanID := time.Now().UTC().Format("20060102T150405.000000000Z07:00")
	startedAt := time.Now().UTC()
	a.setScanState(scanState{Active: true, ScanID: scanID, StartedAt: startedAt})
	go func() {
		db, meta, err := a.buildIndexDatabase(func(update scanState) {
			if update.ScanID == "" {
				update.ScanID = scanID
			}
			if update.StartedAt.IsZero() {
				update.StartedAt = startedAt
			}
			a.setScanState(update)
		})
		if err != nil {
			finished := time.Now().UTC()
			a.setScanState(scanState{
				Active:     false,
				ScanID:     scanID,
				StartedAt:  startedAt,
				FinishedAt: &finished,
				Error:      err.Error(),
			})
			return
		}
		if saveErr := a.saveIndexState(meta); saveErr != nil {
			fmt.Printf("[index] warning: save state: %v\n", saveErr)
		}
		_ = a.swapActiveIndex(db, meta)
	}()

	snapshot := a.CurrentIndexSnapshot()
	return &ReloadLibraryResponse{
		Accepted:     true,
		ScanID:       scanID,
		IndexVersion: snapshot.Version,
		IndexHash:    snapshot.Hash,
		Scanning:     true,
	}, nil
}

type cacheClearResult struct {
	FilesRemoved int    `json:"files_removed"`
	BytesFreed   int64  `json:"bytes_freed"`
	CacheDir     string `json:"cache_dir"`
}

// ClearCache removes all cached files under cacheDir except the index subdirectory.
func (a *App) ClearCache(context.Context) (any, error) {
	var count int
	var size int64

	entries, err := os.ReadDir(a.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("read cache dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "index" {
				continue
			}
			subDir := filepath.Join(a.cacheDir, entry.Name())
			filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					size += info.Size()
					count++
				}
				return nil
			})
			os.RemoveAll(subDir)
		} else {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			size += info.Size()
			count++
			os.Remove(filepath.Join(a.cacheDir, entry.Name()))
		}
	}

	return &cacheClearResult{
		FilesRemoved: count,
		BytesFreed:   size,
		CacheDir:     a.cacheDir,
	}, nil
}

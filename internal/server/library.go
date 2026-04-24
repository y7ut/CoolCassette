package server

import (
	"context"
	"time"
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
func (a *App) ReloadLibrary(context.Context) (any, error) {
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

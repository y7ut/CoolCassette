package server

import (
	"context"
	"os"
	"path/filepath"
)

// AlbumTrackPath resolves an on-disk path for album-local audio files.
func (a *App) AlbumTrackPath(ctx context.Context, id, name string) (string, error) {
	record, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return "", err
	}

	trackPath := filepath.Join(record.Dir, name)
	if _, err := os.Stat(trackPath); err != nil {
		return "", os.ErrNotExist
	}
	return trackPath, nil
}

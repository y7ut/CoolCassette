package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/wampy"
)

// AlbumAssetPath resolves a local asset path for album cover and preview images.
func (a *App) AlbumAssetPath(ctx context.Context, id, name string) (string, error) {
	record, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return "", err
	}

	switch name {
	case "cover.png":
		return a.coverCacheForAlbum(record)
	case "tape.png":
		return filepath.Join(record.Dir, "tape.png"), nil
	case "reel.png":
		return filepath.Join(record.Dir, "reel.png"), nil
	default:
		return "", os.ErrNotExist
	}
}

// PublishedAssetPath resolves a decoded preview path for deployed PKM assets.
func (a *App) PublishedAssetPath(ctx context.Context, id, name string) (string, error) {
	record, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return "", err
	}

	ref, err := wampy.ReadCassette(record.Dir)
	if err != nil {
		return "", err
	}
	if a.cfg.WampyDir == "" {
		return "", os.ErrNotExist
	}

	cacheDir := filepath.Join(a.cacheDir, id)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	var src, dst string
	switch name {
	case "tape.png":
		src = filepath.Join(wampy.TapeDir(a.cfg.WampyDir, ref.Tape), "tape.pkm")
		dst = filepath.Join(cacheDir, ".published_tape.ccimg")
	case "reel.png":
		src = filepath.Join(wampy.ReelDir(a.cfg.WampyDir, ref.Reel), "atlas.pkm")
		dst = filepath.Join(cacheDir, ".published_reel.ccimg")
	default:
		return "", os.ErrNotExist
	}
	if err := a.ensureDecodedPKM(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func (a *App) ensureDecodedPKM(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info, err := os.Stat(dst); err == nil && !info.ModTime().Before(srcInfo.ModTime()) {
		return nil
	}

	cmd := exec.Command(a.etc1toolPath, "--decode", src, "-o", dst)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("decode pkm: %w\n%s", err, string(out))
	}
	return nil
}

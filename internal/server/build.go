package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/api"
	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/config"
	"github.com/coolcassette/coolcassette/internal/deploy"
	reelgen "github.com/coolcassette/coolcassette/internal/reel"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/coolcassette/coolcassette/internal/theme"
)

// GeneratePreview creates or refreshes album-local preview PNG assets.
func (a *App) GeneratePreview(ctx context.Context, id string, request api.ForceRequest) (any, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()

	record, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if request.Force {
		_ = os.Remove(filepath.Join(record.Dir, "tape.png"))
		_ = os.Remove(filepath.Join(record.Dir, "reel.png"))
	}
	if err := a.renderAlbumPreview(ctx, record.Dir, record.FirstAudioFile); err != nil {
		return nil, err
	}
	if err := a.refreshAlbumInActiveIndex(ctx, id); err != nil {
		return nil, err
	}
	return a.GetAlbum(ctx, id)
}

// PublishAlbum builds tape assets for an album and deploys them into Wampy.
func (a *App) PublishAlbum(ctx context.Context, id string, request api.ForceRequest) (any, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()

	record, _, err := a.getAlbumRecordByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.publishAlbum(ctx, record.Dir, record.Slug, record.FirstAudioFile, request.Force); err != nil {
		return nil, err
	}
	if err := a.refreshAlbumInActiveIndex(ctx, id); err != nil {
		return nil, err
	}
	return a.GetAlbum(ctx, id)
}

func (a *App) renderAlbumPreview(ctx context.Context, albumDir, firstAudioFile string) error {
	coverData, err := audio.ExtractCover(firstAudioFile)
	if err != nil {
		return fmt.Errorf("extract cover: %w", err)
	}
	colors, err := theme.Extract(coverData.Data, 5)
	if err != nil {
		return fmt.Errorf("extract colors: %w", err)
	}

	opts := tape.Options{
		Shell:    resolveShell(a.cfg.Shell),
		APIKey:   a.cfg.APIKey,
		Provider: tape.Provider(a.cfg.Provider),
		Verbose:  a.cfg.Verbose,
	}

	tapePath := filepath.Join(albumDir, "tape.png")
	if err := tape.RenderPreviewShellGuided(ctx, coverData.Data, colors, tapePath, a.shellsDir, opts); err != nil {
		return fmt.Errorf("render preview: %w", err)
	}

	reelPath := filepath.Join(albumDir, "reel.png")
	if err := reelgen.Generate(tapePath, reelPath); err != nil {
		return fmt.Errorf("render reel preview: %w", err)
	}
	return nil
}

func (a *App) publishAlbum(ctx context.Context, albumDir, slug, firstAudioFile string, force bool) error {
	if force {
		_ = os.Remove(filepath.Join(albumDir, "tape.png"))
		_ = os.Remove(filepath.Join(albumDir, "reel.png"))
	}

	coverData, err := audio.ExtractCover(firstAudioFile)
	if err != nil {
		return fmt.Errorf("extract cover: %w", err)
	}
	colors, err := theme.Extract(coverData.Data, 5)
	if err != nil {
		return fmt.Errorf("extract colors: %w", err)
	}
	dominant := colors[0]

	workDir, err := os.MkdirTemp("", "coolcassette-server-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	outDir := filepath.Join(workDir, slug)
	resolvedShell := resolveShell(a.cfg.Shell)

	var cachedPNG string
	if coreTape := tape.FindCoreTape(albumDir); coreTape != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("create out dir: %w", err)
		}
		shellPath := filepath.Join(a.shellsDir, fmt.Sprintf("shell_%s.png", resolvedShell))
		pngPath := filepath.Join(outDir, "tape.png")
		if err := tape.CompositeTapePublic(coreTape, shellPath, pngPath); err != nil {
			return fmt.Errorf("composite core_tape: %w", err)
		}
		cachedPNG = pngPath
	} else {
		cachedPNG = filepath.Join(albumDir, "tape.png")
	}

	opts := tape.Options{
		Shell:    resolvedShell,
		APIKey:   a.cfg.APIKey,
		Provider: tape.Provider(a.cfg.Provider),
		Verbose:  a.cfg.Verbose,
	}
	renderResult, err := tape.RenderShellGuided(ctx, coverData.Data, colors, outDir, a.shellsDir, a.etc1toolPath, cachedPNG, opts)
	if err != nil {
		return fmt.Errorf("render tape: %w", err)
	}

	tapePreviewPath := filepath.Join(albumDir, "tape.png")
	if err := copyFile(renderResult.PNGPath, tapePreviewPath); err != nil {
		return fmt.Errorf("cache tape preview: %w", err)
	}

	tapeSlug := slug + "_tape"
	reelSlug := slug + "_reel"
	activeReelName := a.cfg.Reel
	reelPreviewPath := filepath.Join(albumDir, "reel.png")

	if err := reelgen.Generate(renderResult.PNGPath, reelPreviewPath); err != nil {
		reelSlug = ""
	} else if reelSlug != "" {
		reelAtlasDir := filepath.Join(outDir, "reel_atlas")
		if err := reelgen.BuildAtlas(reelPreviewPath, reelAtlasDir, a.etc1toolPath, reelgen.DefaultParams(), reelAnimDelayMS); err != nil {
			return fmt.Errorf("build reel atlas: %w", err)
		}
		if err := deploy.DeployReel(reelAtlasDir, a.cfg.WampyDir, reelSlug); err != nil {
			return fmt.Errorf("deploy reel: %w", err)
		}
		activeReelName = reelSlug
	}

	cfg := config.DefaultConfig(dominant)
	cfg.Reel = activeReelName
	if err := config.WriteTapeConfig(outDir, cfg); err != nil {
		return fmt.Errorf("write tape config: %w", err)
	}
	if err := config.WriteCassetteTxt(albumDir, tapeSlug, activeReelName); err != nil {
		return fmt.Errorf("write cassette.txt: %w", err)
	}
	if err := deploy.DeployTape(outDir, a.cfg.WampyDir, tapeSlug); err != nil {
		return fmt.Errorf("deploy tape: %w", err)
	}

	return nil
}

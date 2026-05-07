package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/config"
	"github.com/coolcassette/coolcassette/internal/preview"
	reelgen "github.com/coolcassette/coolcassette/internal/reel"
	"github.com/coolcassette/coolcassette/internal/scanner"
	"github.com/coolcassette/coolcassette/internal/server"
	shellpkg "github.com/coolcassette/coolcassette/internal/shell"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/coolcassette/coolcassette/internal/theme"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var shareOutputDir string

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Build tape/reel skins into a shareable directory without deploying to a device",
	Long: `Scans the music directory, generates tape and reel skins, and writes them
into a portable directory structure:

  <output-dir>/
    <Artist>/
      <Album>/
        tape/          ← tape skin (tape.pkm + config.txt)
        reel/          ← reel skin (atlas.pkm + atlas.txt + config.txt)
        cassette.txt   ← ready to drop into music directory
        preview.png    ← tape preview image

The output can be shared with others or manually copied to a Wampy device.`,
	RunE: runShare,
}

func init() {
	shareCmd.Flags().StringVarP(&shareOutputDir, "output-dir", "o", "", "output directory (default: ./share)")
	rootCmd.AddCommand(shareCmd)
}

func runShare(cmd *cobra.Command, args []string) error {
	if len(musicDirs) == 0 {
		return fmt.Errorf("--music-dir is required")
	}
	musicDir := musicDirs[0]

	outDir := shareOutputDir
	if outDir == "" {
		outDir = "share"
	}

	etc1toolPath, err := server.ResolveEtc1Tool()
	if err != nil {
		return err
	}

	shellsDir, err := shellpkg.EnsureDir()
	if err != nil {
		return err
	}

	fmt.Printf("Scanning %s ...\n", musicDir)
	albums, err := scanner.Scan(musicDir, force)
	if err != nil {
		return fmt.Errorf("scan music dir: %w", err)
	}
	if len(albums) == 0 {
		fmt.Println("No albums found.")
		return nil
	}
	fmt.Printf("Found %d albums.\n", len(albums))

	if dryRun {
		for _, a := range albums {
			fmt.Printf("  [dry-run] %s → %s\n", a.Name, shareAlbumDir(outDir, a))
		}
		return nil
	}

	workDir, err := os.MkdirTemp("", "coolcassette-share-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	bar := progressbar.NewOptions(len(albums),
		progressbar.OptionSetDescription("Building share"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
	)

	var errs []string
	for _, album := range albums {
		bar.Describe(album.Name)
		if err := processAlbumShare(cmd.Context(), album, workDir, shellsDir, etc1toolPath, outDir); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", album.Name, err))
			if verbose {
				fmt.Fprintf(os.Stderr, "\n[error] %s: %v\n", album.Name, err)
			}
		}
		bar.Add(1)
	}

	fmt.Printf("\nDone. %d/%d albums written to %s\n",
		len(albums)-len(errs), len(albums), outDir)

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\nErrors:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}
	return nil
}

// shareAlbumDir returns the output path for a given album:
//
//	<outDir>/<Artist>/<Album>  (from tags, falling back to slug)
func shareAlbumDir(outDir string, album scanner.Album) string {
	meta := audio.ReadAlbumMeta(album.FirstAudioFile)
	artist := sanitizeDirName(meta.Artist)
	albumName := sanitizeDirName(meta.Album)
	if artist == "" {
		artist = "Unknown Artist"
	}
	if albumName == "" {
		albumName = album.Slug
	}
	return filepath.Join(outDir, artist, albumName)
}

func processAlbumShare(ctx context.Context, album scanner.Album, workDir, shellsDir, etc1toolPath, outDir string) error {
	if verbose {
		fmt.Printf("\n[share] %s\n", album.Name)
	}

	// 1. Extract cover
	coverData, err := audio.ExtractCover(album.FirstAudioFile)
	if err != nil {
		return fmt.Errorf("extract cover: %w", err)
	}

	// 2. Extract dominant colors
	colors, err := theme.Extract(coverData.Data, 5)
	if err != nil {
		return fmt.Errorf("extract colors: %w", err)
	}
	dominant := colors[0]

	// Determine share output directory for this album
	albumOutDir := shareAlbumDir(outDir, album)
	if err := os.MkdirAll(albumOutDir, 0755); err != nil {
		return fmt.Errorf("create album share dir: %w", err)
	}

	resolvedShell := resolveShell()
	tmpDir := filepath.Join(workDir, album.Slug)

	// When --force, discard caches
	if force {
		_ = os.Remove(filepath.Join(album.Dir, "tape.png"))
		_ = os.Remove(filepath.Join(album.Dir, "reel.png"))
	}

	var cachedPNG string
	if coreTape := tape.FindCoreTape(album.Dir); coreTape != "" {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("create tmp dir: %w", err)
		}
		shellPath := filepath.Join(shellsDir, fmt.Sprintf("shell_%s.png", resolvedShell))
		pngPath := filepath.Join(tmpDir, "tape.png")
		if err := tape.CompositeTapePublic(coreTape, shellPath, pngPath); err != nil {
			return fmt.Errorf("composite core_tape: %w", err)
		}
		cachedPNG = pngPath
	} else {
		cachedPNG = filepath.Join(album.Dir, "tape.png")
	}

	opts := tape.Options{
		Shell:    resolvedShell,
		APIKey:   apiKey,
		Provider: tape.Provider(provider),
		BaseURL:  baseURL,
		Model:    model,
		Verbose:  verbose,
	}

	// 3. Render tape (PNG + PKM)
	renderResult, err := tape.RenderShellGuided(ctx, coverData.Data, colors, tmpDir, shellsDir, etc1toolPath, cachedPNG, opts)
	if err != nil {
		return fmt.Errorf("render tape: %w", err)
	}
	defer os.Remove(renderResult.PNGPath)

	tapeSlug := album.Slug + "_tape"
	albumReelSlug := album.Slug + "_reel"

	// 4. Generate reel.png → atlas
	activeReelName := reel
	{
		reelCachePath := filepath.Join(album.Dir, "reel.png")
		reelTmpPath := filepath.Join(tmpDir, "reel.png")
		defer os.Remove(reelTmpPath)
		defer os.Remove(reelCachePath)

		cacheHit := false
		if !force {
			if _, err := os.Stat(reelCachePath); err == nil {
				if err := copyFileLocal(reelCachePath, reelTmpPath); err != nil {
					return fmt.Errorf("copy cached reel: %w", err)
				}
				cacheHit = true
			}
		}
		if !cacheHit {
			if err := reelgen.Generate(renderResult.PNGPath, reelTmpPath); err != nil {
				if verbose {
					fmt.Printf("  [warn] reel generation failed: %v\n", err)
				}
				albumReelSlug = ""
			} else {
				_ = copyFileLocal(reelTmpPath, reelCachePath)
			}
		}

		if albumReelSlug != "" {
			reelAtlasDir := filepath.Join(tmpDir, "reel_atlas")
			reelParams := reelgen.DefaultParams()
			if err := reelgen.BuildAtlas(reelTmpPath, reelAtlasDir, etc1toolPath, reelParams, reelAnimDelayMS); err != nil {
				return fmt.Errorf("build reel atlas: %w", err)
			}
			// Copy reel atlas into share output: reel/<slug>_reel/
			reelShareDir := filepath.Join(albumOutDir, "reel", albumReelSlug)
			if err := copyDirContents(reelAtlasDir, reelShareDir); err != nil {
				return fmt.Errorf("copy reel to share: %w", err)
			}
			activeReelName = albumReelSlug
		}
	}

	// 5. Write config.txt and copy tape skin into share output: tape/<slug>_tape/
	cfg := config.DefaultConfig(dominant)
	cfg.Reel = activeReelName
	tapeShareDir := filepath.Join(albumOutDir, "tape", tapeSlug)
	if err := config.WriteTapeConfig(tapeShareDir, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// Copy tape.pkm into tape share dir
	if err := copyFileLocal(renderResult.PKMPath, filepath.Join(tapeShareDir, "tape.pkm")); err != nil {
		return fmt.Errorf("copy tape.pkm to share: %w", err)
	}

	// 6. Write cassette.txt into share output (album root, not music dir)
	if err := config.WriteCassetteTxt(albumOutDir, tapeSlug, activeReelName); err != nil {
		return fmt.Errorf("write cassette.txt: %w", err)
	}

	// 7. Write self-contained preview.html (tape + reel embedded as base64)
	meta := audio.ReadAlbumMeta(album.FirstAudioFile)
	htmlInfo := preview.Info{
		Artist:     meta.Artist,
		Album:      meta.Album,
		TapeSlug:   tapeSlug,
		ReelSlug:   activeReelName,
		TextColor:  cfg.TextColor,
		ArtistX:    cfg.ArtistX,
		ArtistY:    cfg.ArtistY,
		TitleX:     cfg.TitleX,
		TitleY:     cfg.TitleY,
		AlbumX:     cfg.AlbumX,
		AlbumY:     cfg.AlbumY,
		TitleWidth: cfg.TitleWidth,
	}
	reelTmpForHTML := filepath.Join(tmpDir, "reel.png")
	htmlPath := filepath.Join(albumOutDir, "preview.html")
	if err := preview.WriteHTML(renderResult.PNGPath, reelTmpForHTML, htmlPath, htmlInfo); err != nil {
		// Non-fatal
		if verbose {
			fmt.Printf("  [warn] could not write preview.html: %v\n", err)
		}
	}

	if verbose {
		fmt.Printf("  → %s\n", albumOutDir)
	}
	return nil
}

// copyDirContents copies all files from src into dst (creates dst if needed).
func copyDirContents(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFileLocal(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// sanitizeDirName replaces characters that are invalid in directory names
// while preserving readability (keeps spaces, letters, numbers, hyphens).
func sanitizeDirName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' ||
			r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

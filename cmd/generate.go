package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/config"
	"github.com/coolcassette/coolcassette/internal/deploy"
	reelgen "github.com/coolcassette/coolcassette/internal/reel"
	"github.com/coolcassette/coolcassette/internal/scanner"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/coolcassette/coolcassette/internal/theme"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// reelAnimDelayMS is the per-frame animation delay for reel atlases (ms).
const reelAnimDelayMS = 55

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Scan music directory and generate cassette tape skins for each album",
	RunE:  runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Validate required flags
	if len(musicDirs) == 0 {
		return fmt.Errorf("--music-dir is required")
	}
	musicDir := musicDirs[0]
	if wampyDir == "" {
		return fmt.Errorf("--wampy-dir is required")
	}

	// Resolve etc1tool path (look next to binary, then in PATH)
	etc1toolPath, err := resolveEtc1Tool()
	if err != nil {
		return err
	}

	// Resolve shells directory (embedded assets next to binary)
	shellsDir, err := resolveShellsDir()
	if err != nil {
		return err
	}

	// Scan music directory
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
			fmt.Printf("  [dry-run] %s → slug: %s\n", a.Name, a.Slug)
		}
		return nil
	}

	// Create a temp working directory
	workDir, err := os.MkdirTemp("", "coolcassette-*")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	bar := progressbar.NewOptions(len(albums),
		progressbar.OptionSetDescription("Generating tapes"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
	)

	var errs []string
	for _, album := range albums {
		bar.Describe(album.Name)

		if err := processAlbum(cmd.Context(), album, workDir, shellsDir, etc1toolPath); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", album.Name, err))
			if verbose {
				fmt.Fprintf(os.Stderr, "\n[error] %s: %v\n", album.Name, err)
			}
		}
		bar.Add(1)
	}

	fmt.Printf("\nDone. %d/%d albums processed successfully.\n",
		len(albums)-len(errs), len(albums))

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\nErrors:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	return nil
}

func processAlbum(ctx context.Context, album scanner.Album, workDir, shellsDir, etc1toolPath string) error {
	if verbose {
		fmt.Printf("\n[process] %s\n", album.Name)
	}

	// 1. Extract cover from first audio file
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

	// 3. Generate tape skin (sticker → PNG → PKM)
	//
	// Priority order:
	//   1. core_tape.{png,jpg,jpeg} in album dir → manual sticker, composite directly onto shell
	//   2. tape.png in album dir                 → cached preview, skip API, encode PKM directly
	//   3. (none)                                → call image API to generate sticker
	outDir := filepath.Join(workDir, album.Slug)

	// Resolve shell once per album (supports "random")
	resolvedShell := resolveShell()
	if verbose {
		fmt.Printf("  [shell] %s\n", resolvedShell)
	}

	// When --force is set, discard any cached tape.png and reel.png in album dir
	// so everything is regenerated from scratch.
	if force {
		_ = os.Remove(filepath.Join(album.Dir, "tape.png"))
		_ = os.Remove(filepath.Join(album.Dir, "reel.png"))
	}

	var cachedPNG string
	if coreTape := tape.FindCoreTape(album.Dir); coreTape != "" {
		if verbose {
			fmt.Printf("  [manual] using core_tape sticker: %s\n", coreTape)
		}
		// Composite core_tape onto shell → tape.png in outDir, then encode PKM
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("create out dir: %w", err)
		}
		shellPath := filepath.Join(shellsDir, fmt.Sprintf("shell_%s.png", resolvedShell))
		pngPath := filepath.Join(outDir, "tape.png")
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
		Verbose:  verbose,
	}

	renderResult, err := tape.RenderShellGuided(ctx, coverData.Data, colors, outDir, shellsDir, etc1toolPath, cachedPNG, opts)
	if err != nil {
		return fmt.Errorf("render tape: %w", err)
	}
	// tape.png is kept alive by renderer; clean it up at the end of this function
	defer os.Remove(renderResult.PNGPath)

	// Slug suffixes to avoid name collisions between tape and reel skin directories
	tapeSlug := album.Slug + "_tape"
	albumReelSlug := album.Slug + "_reel"

	// 4. Generate reel.png from the tape.png that renderer just produced
	activeReelName := reel // default: global reel flag (fallback if reel generation fails)
	{
		reelCachePath := filepath.Join(album.Dir, "reel.png")
		reelOutPath := filepath.Join(outDir, "reel.png")
		defer os.Remove(reelOutPath)

		// Use cached reel.png from album dir (written by 'preview') if present,
		// unless --force is set in which case we always regenerate.
		cacheHit := false
		if !force {
			if _, err := os.Stat(reelCachePath); err == nil {
				if verbose {
					fmt.Printf("  [cache] reusing existing reel.png: %s\n", reelCachePath)
				}
				if err := copyFileLocal(reelCachePath, reelOutPath); err != nil {
					return fmt.Errorf("copy cached reel: %w", err)
				}
				cacheHit = true
			}
		}

		if !cacheHit {
			if verbose {
				fmt.Printf("  [reel] generating from %s\n", renderResult.PNGPath)
			}
			if err := reelgen.Generate(renderResult.PNGPath, reelOutPath); err != nil {
				// Non-fatal: fall back to global reel setting
				if verbose {
					fmt.Printf("  [warn] reel generation failed: %v\n", err)
				}
				albumReelSlug = "" // mark as failed
			}
		}

		// Always clean up the cached reel.png from album dir after use —
		// it will be regenerated from tape.png on the next run if needed.
		defer os.Remove(reelCachePath)

		// Build atlas (pkm + atlas.txt + config.txt) and deploy
		if albumReelSlug != "" {
			reelAtlasDir := filepath.Join(outDir, "reel_atlas")
			reelParams := reelgen.DefaultParams()
			if err := reelgen.BuildAtlas(reelOutPath, reelAtlasDir, etc1toolPath, reelParams, reelAnimDelayMS); err != nil {
				return fmt.Errorf("build reel atlas: %w", err)
			}
			if err := deploy.DeployReel(reelAtlasDir, wampyDir, albumReelSlug); err != nil {
				return fmt.Errorf("deploy reel: %w", err)
			}
			activeReelName = albumReelSlug
			if verbose {
				fmt.Printf("  → deployed reel: wampy/skins/cassette/reel/%s/\n", albumReelSlug)
			}
		}
	}

	// 5. Write config.txt (with the album-specific reel name)
	cfg := config.DefaultConfig(dominant)
	cfg.Reel = activeReelName
	if err := config.WriteTapeConfig(outDir, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// 6. Write cassette.txt into the album's music directory
	if err := config.WriteCassetteTxt(album.Dir, tapeSlug, activeReelName); err != nil {
		return fmt.Errorf("write cassette.txt: %w", err)
	}

	// 7. Deploy tape skin to wampy directory (using _tape suffix)
	if err := deploy.DeployTape(outDir, wampyDir, tapeSlug); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	if verbose {
		fmt.Printf("  → deployed tape: wampy/skins/cassette/tape/%s/\n", tapeSlug)
	}
	return nil
}

// copyFileLocal is a local copy helper for generate.go
func copyFileLocal(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// resolveEtc1Tool finds etc1tool next to the binary or falls back to PATH.
func resolveEtc1Tool() (string, error) {
	// Check next to binary (platform-tools/etc1tool)
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "platform-tools", "etc1tool")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Also check same dir as binary
		candidate = filepath.Join(filepath.Dir(exe), "etc1tool")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Check in current working directory (dev mode)
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "platform-tools", "etc1tool")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	candidate = filepath.Join(cwd, "etc1tool")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("etc1tool not found. Download Android platform-tools from:\nhttps://dl.google.com/android/repository/platform-tools-latest-darwin.zip\nand place etc1tool next to the coolcassette binary.")
}

// resolveShellsDir finds the assets/templates directory.
func resolveShellsDir() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "assets", "templates")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Dev mode: look in cwd
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "assets", "templates")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	// Also try templates/ directly (project root)
	candidate = filepath.Join(cwd, "templates")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("shell templates not found. Expected assets/templates/ directory.")
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/config"
	"github.com/coolcassette/coolcassette/internal/deploy"
	"github.com/coolcassette/coolcassette/internal/scanner"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/coolcassette/coolcassette/internal/theme"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

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
	if musicDir == "" {
		return fmt.Errorf("--music-dir is required")
	}
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
	albums, err := scanner.Scan(musicDir)
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
		Method:   tape.Method(method),
		Verbose:  verbose,
	}

	switch tape.Method(method) {
	case tape.MethodShellGuided:
		_, err = tape.RenderShellGuided(ctx, coverData.Data, colors, outDir, shellsDir, etc1toolPath, cachedPNG, opts)
	default:
		_, err = tape.Render(ctx, coverData.Data, colors, outDir, shellsDir, etc1toolPath, cachedPNG, opts)
	}
	if err != nil {
		return fmt.Errorf("render tape: %w", err)
	}

	// 4. Write config.txt
	cfg := config.DefaultConfig(dominant)
	cfg.Reel = reel
	if err := config.WriteTapeConfig(outDir, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// 5. Write cassette.txt into the album's music directory
	if err := config.WriteCassetteTxt(album.Dir, album.Slug, reel); err != nil {
		return fmt.Errorf("write cassette.txt: %w", err)
	}

	// 6. Deploy to wampy directory
	if err := deploy.DeployTape(outDir, wampyDir, album.Slug); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	if verbose {
		fmt.Printf("  → deployed: wampy/skins/cassette/tape/%s/\n", album.Slug)
	}
	return nil
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

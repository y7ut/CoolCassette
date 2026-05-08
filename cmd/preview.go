package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/audio"
	reelgen "github.com/coolcassette/coolcassette/internal/reel"
	"github.com/coolcassette/coolcassette/internal/scanner"
	"github.com/coolcassette/coolcassette/internal/server"
	shellpkg "github.com/coolcassette/coolcassette/internal/shell"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/coolcassette/coolcassette/internal/theme"
	"github.com/spf13/cobra"
)

var (
	previewOutput string
)

var previewCmd = &cobra.Command{
	Use:   "preview [album-dir]",
	Short: "Generate a preview tape PNG for a single album directory",
	Long: `Generate a tape.png preview for a single album directory.

The output is saved as tape.png inside the album directory by default.
When you later run 'generate', if tape.png already exists in the album
directory it will be used directly (skipping the API call) and compressed
to tape.pkm for deployment.`,
	Args: cobra.ExactArgs(1),
	RunE: runPreview,
}

func init() {
	previewCmd.Flags().StringVarP(&previewOutput, "output", "o", "", "output PNG path (default: <album-dir>/tape.png)")
	rootCmd.AddCommand(previewCmd)
}

func runPreview(cmd *cobra.Command, args []string) error {
	albumDir := args[0]

	shellsDir, err := shellpkg.EnsureDir()
	if err != nil {
		return err
	}

	// Find first audio file in the given directory
	albums, err := scanner.Scan(filepath.Dir(albumDir), true)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	var targetAlbum *scanner.Album
	for i, a := range albums {
		if a.Dir == albumDir || filepath.Base(a.Dir) == filepath.Base(albumDir) {
			targetAlbum = &albums[i]
			break
		}
	}
	if targetAlbum == nil {
		return fmt.Errorf("no supported audio files found in %s", albumDir)
	}

	// Extract cover
	coverData, err := audio.ExtractCover(targetAlbum.FirstAudioFile)
	if err != nil {
		return fmt.Errorf("extract cover: %w", err)
	}

	// Extract colors
	colors, err := theme.Extract(coverData.Data, 5)
	if err != nil {
		return fmt.Errorf("extract colors: %w", err)
	}

	// Determine output path — default to tape.png so generate can reuse it
	outPath := previewOutput
	if outPath == "" {
		outPath = filepath.Join(albumDir, "tape.png")
	}

	fmt.Printf("Generating preview for: %s\n", targetAlbum.Name)
	fmt.Printf("Cover: %s\n", targetAlbum.FirstAudioFile)
	fmt.Printf("Dominant color: %s\n", colors[0].Hex())

	if dryRun {
		fmt.Printf("[dry-run] would write: %s\n", outPath)
		return nil
	}

	resolvedShell := resolveShell()
	opts := tape.Options{
		Shell:    resolvedShell,
		APIKey:   apiKey,
		Provider: tape.Provider(provider),
		BaseURL:  baseURL,
		Model:    model,
		Verbose:  verbose,
	}

	fmt.Println("Method: shell-guided (cover + shell template → AI)")
	if err := tape.RenderPreviewShellGuided(
		context.Background(),
		coverData.Data,
		colors,
		outPath,
		shellsDir,
		opts,
	); err != nil {
		return fmt.Errorf("render preview: %w", err)
	}

	fmt.Printf("Preview saved: %s\n", outPath)

	// Generate reel.png alongside tape.png for preview inspection.
	// generate will consume and delete it automatically.
	reelOutPath := filepath.Join(albumDir, "reel.png")
	fmt.Printf("Generating reel animation: %s\n", reelOutPath)
	if err := reelgen.Generate(outPath, reelOutPath); err != nil {
		fmt.Fprintf(os.Stderr, "  [warn] reel generation failed: %v\n", err)
	} else {
		fmt.Printf("Reel saved: %s\n", reelOutPath)
	}

	fmt.Printf("\nRun 'generate' to reuse this preview (skips API call):\n")
	fmt.Printf("  coolcassette generate --music-dir %s --wampy-dir <wampy-path>\n", filepath.Dir(albumDir))

	// Print color palette
	fmt.Println("\nColor palette:")
	for i, c := range colors {
		fmt.Printf("  %d. %s  weight=%.1f%%\n", i+1, c.Hex(), c.Weight*100)
	}
	fmt.Printf("\nText color: %s\n", colors[0].TextColor())

	// Print prompt snippet
	fmt.Printf("\nPrompt color hint:\n  The dominant colors are %s\n",
		theme.BuildColorDescription(colors))

	// Notify about etc1tool for final deploy
	fmt.Printf("\nTo compress for Wampy:\n")
	etc1, _ := server.ResolveEtc1Tool()
	if etc1 == "" {
		etc1 = "./platform-tools/etc1tool"
	}
	pkmPath := outPath[:len(outPath)-4] + ".pkm"
	fmt.Printf("  %s %s -o %s\n", etc1, outPath, pkmPath)

	return nil
}

// printAlbumInfo prints details about a found album.
func printAlbumInfo(a scanner.Album) {
	fmt.Printf("  Album:  %s\n", a.Name)
	fmt.Printf("  Slug:   %s\n", a.Slug)
	fmt.Printf("  Audio:  %s\n", filepath.Base(a.FirstAudioFile))
	info, err := os.Stat(a.FirstAudioFile)
	if err == nil {
		fmt.Printf("  Size:   %d KB\n", info.Size()/1024)
	}
}

package cmd

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/spf13/cobra"
)

var shells = []string{"chf", "bhf"}

// resolveShell returns the shell to use, picking randomly if shell == "random".
func resolveShell() string {
	if shell == "random" {
		return shells[rand.Intn(len(shells))]
	}
	return shell
}

var (
	musicDir string
	wampyDir string
	reel     string
	dryRun   bool
	force    bool
	verbose  bool
	apiKey   string
	shell    string
	provider string
)

var rootCmd = &cobra.Command{
	Use:   "coolcassette",
	Short: "Generate Wampy cassette tape skins from your music library",
	Long: `CoolCassette scans your music directory, extracts album artwork,
generates cassette tape sticker images using AI, and deploys them
to your Wampy player directory on your NW-series Walkman.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&musicDir, "music-dir", "", "path to music root directory (required)")
	rootCmd.PersistentFlags().StringVar(&wampyDir, "wampy-dir", "", "path to wampy directory on device (required for deploy)")
	rootCmd.PersistentFlags().StringVar(&reel, "reel", "other", "reel name to use (default: other)")
	rootCmd.PersistentFlags().StringVar(&shell, "shell", "random", "cassette shell template: chf, bhf, or random")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "openrouter", "image generation provider: openrouter, openai, or google")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for image generation (OPENROUTER_API_KEY or OPENAI_API_KEY env)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print plan without writing any files")
	rootCmd.PersistentFlags().BoolVar(&force, "force", false, "reprocess albums that already have cassette.txt, ignoring all caches")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
}

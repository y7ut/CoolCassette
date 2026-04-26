package cmd

import (
	"fmt"
	"math/rand"
	"os"

	ccserver "github.com/coolcassette/coolcassette/internal/server"
	"github.com/spf13/cobra"
)

var shells = []string{"chf", "bhf"}

func resolveShell() string {
	if shell == "random" {
		return shells[rand.Intn(len(shells))]
	}
	return shell
}

var (
	musicDirs []string
	wampyDir  string
	reel      string
	dryRun    bool
	force     bool
	verbose   bool
	apiKey    string
	shell     string
	provider  string
)

var userCfg ccserver.UserConfig

var rootCmd = &cobra.Command{
	Use:   "coolcassette",
	Short: "Generate Wampy cassette tape skins from your music library",
	Long: `CoolCassette scans your music directory, extracts album artwork,
generates cassette tape sticker images using AI, and deploys them
to your Wampy player directory on your NW-series Walkman.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if apiKey == "" {
			if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
				apiKey = v
			} else if v := os.Getenv("API_KEY"); v != "" {
				apiKey = v
			} else if userCfg.APIKey != "" {
				apiKey = userCfg.APIKey
			}
		}
		if v, _ := cmd.Flags().GetString("provider"); v == "openrouter" && userCfg.Provider != "" {
			provider = userCfg.Provider
		}
	},
}

func Execute() {
	userCfg = ccserver.LoadUserConfig()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringArrayVar(&musicDirs, "music-dir", nil, "path to music root directory (repeatable)")
	rootCmd.PersistentFlags().StringVar(&wampyDir, "wampy-dir", "", "path to wampy directory on device (required for deploy)")
	rootCmd.PersistentFlags().StringVar(&reel, "reel", "other", "reel name to use (default: other)")
	rootCmd.PersistentFlags().StringVar(&shell, "shell", "random", "cassette shell template: chf, bhf, or random")
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "openrouter", "image generation provider: openrouter, openai, or google")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for image generation (also ~/.coolcassette.json)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print plan without writing any files")
	rootCmd.PersistentFlags().BoolVar(&force, "force", false, "reprocess albums that already have cassette.txt, ignoring all caches")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
}

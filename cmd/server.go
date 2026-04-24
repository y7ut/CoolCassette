package cmd

import (
	"fmt"

	ccserver "github.com/coolcassette/coolcassette/internal/server"
	"github.com/spf13/cobra"
)

var serverListen string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the CoolCassette API server",
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringVar(&serverListen, "listen", "127.0.0.1:7350", "listen address")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	if musicDir == "" {
		return fmt.Errorf("--music-dir is required")
	}
	if wampyDir == "" {
		return fmt.Errorf("--wampy-dir is required")
	}

	app, err := ccserver.New(ccserver.Config{
		MusicDir: musicDir,
		WampyDir: wampyDir,
		APIKey:   apiKey,
		Provider: provider,
		Shell:    shell,
		Reel:     reel,
		Verbose:  verbose,
		Listen:   serverListen,
	})
	if err != nil {
		return err
	}

	engine := app.NewEngine()
	fmt.Printf("CoolCassette server listening on http://%s\n", serverListen)
	fmt.Printf("Health check: http://%s/api/health\n", serverListen)
	if err := engine.Run(serverListen); err != nil {
		return err
	}
	return nil
}

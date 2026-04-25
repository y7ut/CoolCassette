package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	app, err := ccserver.New(ccserver.Config{
		MusicDirs: musicDirs,
		WampyDir:  wampyDir,
		APIKey:    apiKey,
		Provider:  provider,
		Shell:     shell,
		Reel:      reel,
		Verbose:   verbose,
		Force:     force,
		Listen:    serverListen,
	})
	if err != nil {
		return err
	}
	defer app.Close()

	engine := app.NewEngine()
	srv := &http.Server{Addr: serverListen, Handler: engine}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		fmt.Println("\nShutting down...")
		srv.Shutdown(context.Background())
	}()

	fmt.Printf("CoolCassette server listening on http://%s\n", serverListen)
	fmt.Printf("Health check: http://%s/api/health\n", serverListen)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

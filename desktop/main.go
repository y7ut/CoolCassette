package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	ccserver "github.com/coolcassette/coolcassette/internal/server"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend_dist
var frontendAssets embed.FS

func initLogger() *log.Logger {
	exe, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(exe), "..", "Logs")
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "coolcassette.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		f, _ = os.OpenFile(filepath.Join(os.TempDir(), "coolcassette.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}

	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("=== CoolCassette started at %s ===", time.Now().Format(time.RFC3339))
	logger.Printf("executable: %s", exe)
	logger.Printf("cwd: %s", func() string { cwd, _ := os.Getwd(); return cwd }())
	return logger
}

func main() {
	lg := initLogger()

	addr := "127.0.0.1:7350"
	if v := os.Getenv("COOLCASSETTE_LISTEN"); v != "" {
		addr = v
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		lg.Fatalf("listen %s: %v", addr, err)
	}
	lg.Printf("listening on %s", listener.Addr().String())

	musicDirs := parseMultiEnv("COOLCASSETTE_MUSIC_DIRS", "MUSIC_DIR")
	wampyDir := envOr("WAMPY_DIR", "")
	lg.Printf("music_dirs=%v wampy_dir=%q", musicDirs, wampyDir)

	srvApp, err := ccserver.New(ccserver.Config{
		MusicDirs: musicDirs,
		WampyDir:  wampyDir,
		APIKey:    envOr("API_KEY", ""),
		Provider:  envOr("PROVIDER", "openrouter"),
		Shell:     envOr("SHELL", "random"),
		Verbose:   os.Getenv("VERBOSE") != "",
	})
	if err != nil {
		lg.Fatalf("server init: %v", err)
	}
	lg.Printf("server initialized")

	engine := srvApp.NewEngine()
	srv := &http.Server{Handler: engine}

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			lg.Printf("server error: %v", err)
		}
	}()

	lg.Printf("starting wails window...")

	distFS, _ := fs.Sub(frontendAssets, "frontend_dist")
	app := &App{svc: srvApp}
	err = wails.Run(&options.App{
		Title:     "CoolCassette",
		Width:     1024,
		Height:    720,
		MinWidth:  800,
		MinHeight: 600,
		Bind: []interface{}{
			app,
		},
		AssetServer: &assetserver.Options{
			Assets: distFS,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				lg.Printf("[handler] %s %s", r.Method, r.URL.Path)
				engine.ServeHTTP(w, r)
			}),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					lg.Printf("[mw] %s %s  host=%s", r.Method, r.URL.Path, r.Host)
					if strings.HasPrefix(r.URL.Path, "/api/") {
						engine.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		OnStartup:  app.startup,
		OnShutdown: func(ctx context.Context) {
			lg.Printf("shutting down...")
			srv.Shutdown(ctx)
			srvApp.Close()
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
			},
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.coolcassette.desktop",
		},
	})
	if err != nil {
		lg.Fatalf("wails run: %v", err)
	}
	lg.Printf("exited cleanly")
}

func envOr(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func parseMultiEnv(keys ...string) []string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			var result []string
			for _, d := range splitBy(v, ",", ":") {
				d = trimSpace(d)
				if d != "" {
					result = append(result, d)
				}
			}
			return result
		}
	}
	return nil
}

func splitBy(s string, seps ...string) []string {
	result := []string{s}
	for _, sep := range seps {
		var expanded []string
		for _, part := range result {
			for _, sub := range split(part, sep) {
				expanded = append(expanded, sub)
			}
		}
		result = expanded
	}
	return result
}

func split(s, sep string) []string {
	var result []string
	for {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

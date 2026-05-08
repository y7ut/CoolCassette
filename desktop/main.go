package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	logDir := filepath.Join(filepath.Dir(exe), ".", "log")
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

	uc := ccserver.LoadUserConfig()
	lg.Printf("user_config: %s", uc)
	musicDirs := parseMultiEnv("COOLCASSETTE_MUSIC_DIRS", "MUSIC_DIR")
	wampyDir := envOr("WAMPY_DIR", "")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CUSTOM_API_KEY")
	}
	if apiKey == "" {
		apiKey = uc.APIKey
	}
	baseURL := os.Getenv("CUSTOM_BASE_URL")
	if baseURL == "" {
		baseURL = uc.BaseURL
	}
	model := os.Getenv("CUSTOM_MODEL")
	if model == "" {
		model = uc.Model
	}
	provider := os.Getenv("PROVIDER")
	if provider == "" {
		provider = uc.Provider
	}
	if provider == "" {
		provider = "custom"
	}
	lg.Printf("music_dirs=%v wampy_dir=%q provider=%s base_url=%s model=%s api_key_set=%v", musicDirs, wampyDir, provider, baseURL, model, apiKey != "")
	
	srvApp, err := ccserver.New(ccserver.Config{
		MusicDirs: musicDirs,
		WampyDir:  wampyDir,
		APIKey:    apiKey,
		Provider:  provider,
		BaseURL:   baseURL,
		Model:     model,
		Shell:     envOr("SHELL", "random"),
		Verbose:   os.Getenv("VERBOSE") != "",
	})
	if err != nil {
		lg.Fatalf("server init: %v", err)
	}
	lg.Printf("server initialized")

	distFS, _ := fs.Sub(frontendAssets, "frontend_dist")
	app := &App{svc: srvApp, lg: lg}
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
			Middleware: func(next http.Handler) http.Handler {
				fh := newFileHandler(lg, srvApp)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/api/albums/") {
						fh.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		OnStartup: app.startup,
		OnShutdown: func(_ context.Context) {
			lg.Printf("shutting down...")
			srvApp.Close()
		},
		Frameless: runtime.GOOS == "windows",
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

func newFileHandler(lg *log.Logger, svc *ccserver.App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if !strings.HasPrefix(path, "/api/albums/") {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(strings.TrimPrefix(path, "/api/albums/"), "/")
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}

		id := parts[0]
		kind := parts[1]
		name := strings.Join(parts[2:], "/")
		var filePath string
		var err error

		switch kind {
		case "assets":
			filePath, err = svc.AlbumAssetPath(r.Context(), id, name)
		case "published":
			filePath, err = svc.PublishedAssetPath(r.Context(), id, name)
		case "tracks":
			filePath, err = svc.AlbumTrackPath(r.Context(), id, name)
		default:
			http.NotFound(w, r)
			return
		}

		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filePath)
	})
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

package server

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coolcassette/coolcassette/api"
	"github.com/coolcassette/coolcassette/internal/audio"
	"github.com/coolcassette/coolcassette/internal/reel"
	"github.com/coolcassette/coolcassette/internal/shell"
	"github.com/coolcassette/coolcassette/internal/tape"
	"github.com/gin-gonic/gin"
)

// Config defines the runtime configuration for the HTTP server and indexer.
type Config struct {
	MusicDirs []string
	WampyDir  string
	APIKey    string
	Provider  string
	Shell     string
	Reel      string
	Verbose   bool
	Listen    string
	Force     bool
}

// App owns the HTTP handlers, active index database, and background scan state.
type App struct {
	cfg          Config
	etc1toolPath string
	shellsDir    string
	cacheDir     string
	indexDir     string
	buildMu      sync.Mutex
	indexMu      sync.RWMutex
	activeDB     *sql.DB
	activeMeta   indexMetadata
	scanMu       sync.RWMutex
	scanState    scanState
}

// New constructs an App, resolves tool paths, and builds the initial SQLite index.
func New(cfg Config) (*App, error) {
	etc1toolPath, err := ResolveEtc1Tool()
	if err != nil {
		return nil, err
	}
	magickPath := ResolveMagick()
	audio.SetMagickPath(magickPath)
	tape.SetMagickPath(magickPath)
	reel.SetMagickPath(magickPath)
	shellsDir, err := shell.EnsureDir()
	if err != nil {
		return nil, fmt.Errorf("extract shell templates: %w", err)
	}
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user cache dir: %w", err)
	}

	cacheDir := filepath.Join(userCacheDir, "coolcassette", ".cccache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	indexDir := filepath.Join(cacheDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("create index dir: %w", err)
	}

	app := &App{
		cfg:          cfg,
		etc1toolPath: etc1toolPath,
		shellsDir:    shellsDir,
		cacheDir:     cacheDir,
		indexDir:     indexDir,
	}
	if err := app.loadInitialIndex(); err != nil {
		return nil, err
	}
	return app, nil
}

// Close checkpoints the active SQLite database and closes the connection.
func (a *App) Close() {
	a.indexMu.Lock()
	defer a.indexMu.Unlock()
	if a.activeDB != nil {
		fmt.Print("Closing index database\n")
		_, _ = a.activeDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		_ = a.activeDB.Close()
		a.activeDB = nil
	}
}

// NewEngine creates the Gin engine used by the API server.
func (a *App) NewEngine() *gin.Engine {
	engine := gin.Default()
	a.RegisterRoutes(engine)
	return engine
}

// RegisterRoutes attaches all API routes to the provided Gin router.
func (a *App) RegisterRoutes(engine gin.IRouter) {
	api.RegisterRoutes(engine, a)
}

package server

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coolcassette/coolcassette/api"
	"github.com/gin-gonic/gin"
)

// Config defines the runtime configuration for the HTTP server and indexer.
type Config struct {
	MusicDir string
	WampyDir string
	APIKey   string
	Provider string
	Shell    string
	Reel     string
	Verbose  bool
	Listen   string
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
	if cfg.MusicDir == "" {
		return nil, fmt.Errorf("music dir is required")
	}
	if cfg.WampyDir == "" {
		return nil, fmt.Errorf("wampy dir is required")
	}

	etc1toolPath, err := resolveEtc1Tool()
	if err != nil {
		return nil, err
	}
	shellsDir, err := resolveShellsDir()
	if err != nil {
		return nil, err
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

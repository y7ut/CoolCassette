package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coolcassette/coolcassette/api"
	ccserver "github.com/coolcassette/coolcassette/internal/server"
)

type App struct {
	ctx context.Context
	svc *ccserver.App
	lg  *log.Logger
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetLibraryStatus() (any, error) {
	return a.svc.GetLibraryStatus(a.ctx)
}

func (a *App) ReloadLibrary(req api.ReloadRequest) (any, error) {
	return a.svc.ReloadLibrary(a.ctx, req)
}

func (a *App) ClearCache() (any, error) {
	return a.svc.ClearCache(a.ctx)
}

func (a *App) BrowseFS(dirPath string) (any, error) {
	return browseFS(dirPath), nil
}

func (a *App) ListAlbums(limit int, sortBy, order, cursor string) (any, error) {
	snap := a.svc.CurrentIndexSnapshot()
	return a.svc.ListAlbums(a.ctx, api.ListAlbumsRequest{
		AlbumListQuery: api.AlbumListQuery{
			Limit:  limit,
			SortBy: sortBy,
			Order:  order,
			Cursor: cursor,
		},
		IndexVersion: snap.Version,
		IndexHash:    snap.Hash,
	})
}

func (a *App) GetAlbum(id string) (any, error) {
	result, err := a.svc.GetAlbum(a.ctx, id)
	if err != nil {
		a.lg.Printf("IPC GetAlbum id=%s error: %v", id, err)
	}
	return result, err
}

func (a *App) GeneratePreview(id string, force bool) (any, error) {
	a.lg.Printf("IPC GeneratePreview id=%s force=%v", id, force)
	result, err := a.svc.GeneratePreview(a.ctx, id, api.ForceRequest{Force: force})
	if err != nil {
		a.lg.Printf("IPC GeneratePreview error: %v", err)
	}
	return result, err
}

func (a *App) PublishAlbum(id string, force bool) (any, error) {
	a.lg.Printf("IPC PublishAlbum id=%s force=%v", id, force)
	result, err := a.svc.PublishAlbum(a.ctx, id, api.ForceRequest{Force: force})
	if err != nil {
		a.lg.Printf("IPC PublishAlbum error: %v", err)
	}
	return result, err
}

type fsEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type fsBrowseResponse struct {
	Path    string    `json:"path"`
	Parent  string    `json:"parent"`
	Entries []fsEntry `json:"entries"`
}

func browseFS(dir string) *fsBrowseResponse {
	if dir == "" {
		dir = "/"
	}
	dir = filepath.Clean(dir)
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	resp := &fsBrowseResponse{Path: dir, Parent: parent}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return resp
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.IsDir() {
			continue
		}
		resp.Entries = append(resp.Entries, fsEntry{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: true,
		})
	}
	sort.Slice(resp.Entries, func(i, j int) bool {
		return strings.ToLower(resp.Entries[i].Name) < strings.ToLower(resp.Entries[j].Name)
	})
	return resp
}

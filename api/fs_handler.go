package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

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

// FSHandler serves filesystem browsing endpoints.
type FSHandler struct{}

// NewFSHandler creates a FSHandler.
func NewFSHandler() *FSHandler {
	return &FSHandler{}
}

// Browse handles GET /api/fs/browse?path=/Users.
func (h *FSHandler) Browse(c *gin.Context) {
	dir := c.Query("path")
	if dir == "" {
		dir = "/"
	}

	dir = filepath.Clean(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dirs []fsEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		dirs = append(dirs, fsEntry{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: true,
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}

	c.JSON(http.StatusOK, fsBrowseResponse{
		Path:    dir,
		Parent:  parent,
		Entries: dirs,
	})
}

package shell

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed shell_chf.png shell_bhf.png
var templates embed.FS

var (
	cachedDir string
	once      sync.Once
)

func EnsureDir() (string, error) {
	var err error
	once.Do(func() {
		var cacheDir string
		cacheDir, err = os.UserCacheDir()
		if err != nil {
			return
		}
		dir := filepath.Join(cacheDir, "coolcassette", "shell-templates")
		if err = os.MkdirAll(dir, 0755); err != nil {
			return
		}
		err = fs.WalkDir(templates, ".", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			data, readErr := fs.ReadFile(templates, path)
			if readErr != nil {
				return readErr
			}
			return os.WriteFile(filepath.Join(dir, path), data, 0644)
		})
		if err == nil {
			cachedDir = dir
		}
	})
	return cachedDir, err
}

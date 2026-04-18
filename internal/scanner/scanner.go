package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// Album represents a music album directory.
type Album struct {
	// Dir is the full path to the album directory.
	Dir string
	// Name is the directory name (used as slug).
	Name string
	// Slug is the sanitized name for use in file paths.
	Slug string
	// FirstAudioFile is the path to the first supported audio file found.
	FirstAudioFile string
}

var supportedExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".wav":  true,
}

// Scan walks musicDir recursively and returns one Album per directory
// that directly contains at least one supported audio file.
//
// Supports flat and nested layouts, e.g.:
//
//	MUSIC/Nujabes - Modal Soul/*.flac         → 1 album
//	MUSIC/Arcade Fire/Neon Bible/*.mp3        → 1 album
//	MUSIC/Arcade Fire/The Arcade Fire/*.flac  → 1 album
func Scan(musicDir string) ([]Album, error) {
	var albums []Album

	err := filepath.WalkDir(musicDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}
		// Skip the root itself
		if path == musicDir {
			return nil
		}

		audioFile, err := findFirstAudio(path)
		if err != nil || audioFile == "" {
			return nil // no audio here, keep walking
		}

		// Skip if cassette.txt already exists — already processed.
		if _, err := os.Stat(filepath.Join(path, "cassette.txt")); err == nil {
			return filepath.SkipDir
		}

		// This directory contains audio files — treat as an album.
		// Use path relative to musicDir as the display name so nested
		// paths like "Arcade Fire/Neon Bible" are human-readable.
		rel, _ := filepath.Rel(musicDir, path)
		slug := sanitizeSlug(rel)

		albums = append(albums, Album{
			Dir:            path,
			Name:           rel,
			Slug:           slug,
			FirstAudioFile: audioFile,
		})

		// Don't descend into subdirectories of an album dir —
		// if this dir already has audio files it IS the album leaf.
		return filepath.SkipDir
	})

	return albums, err
}

// findFirstAudio returns the first supported audio file directly in dir (non-recursive).
func findFirstAudio(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if supportedExts[ext] {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", nil
}

// sanitizeSlug converts a path (possibly with separators) to a safe slug.
// e.g. "Arcade Fire/Neon Bible" → "arcade_fire_neon_bible"
func sanitizeSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			// Replace path separators and special chars with underscore
			b.WriteRune('_')
		}
	}
	// Collapse multiple underscores
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coolcassette/coolcassette/internal/theme"
)

// TapeConfig holds all parameters for a tape's config.txt.
type TapeConfig struct {
	Reel           string
	ArtistX        float64
	ArtistY        float64
	ArtistFormat   string
	TitleX         float64
	TitleY         float64
	TitleFormat    string
	AlbumX         float64
	AlbumY         float64
	AlbumFormat    string
	ReelX          float64
	ReelY          float64
	TitleWidth     float64
	DurationFormat string
	TextColor      string
}

// DefaultConfig returns a sensible default TapeConfig for the given dominant color.
func DefaultConfig(dominant theme.Color) TapeConfig {
	return TapeConfig{
		Reel:           "other",
		ArtistX:        83.0,
		ArtistY:        65.0,
		ArtistFormat:   "$ARTIST",
		TitleX:         83.0,
		TitleY:         95.0,
		TitleFormat:    "$TITLE",
		AlbumX:         -1.0,
		AlbumY:         -1.0,
		AlbumFormat:    "$ALBUM",
		ReelX:          180.0,
		ReelY:          161.0,
		TitleWidth:     580.0,
		DurationFormat: "%1$02d:%2$02d",
		TextColor:      dominant.TextColor(),
	}
}

// WriteTapeConfig writes config.txt to the given directory.
func WriteTapeConfig(dir string, cfg TapeConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	content := fmt.Sprintf(
		"# default reel\n"+
			"reel: %s\n"+
			"# track artist coordinates\n"+
			"artistx: %.1f\n"+
			"artisty: %.1f\n"+
			"# artist format string\n"+
			"artistformat: %s\n"+
			"# track title coordinates\n"+
			"titlex: %.1f\n"+
			"titley: %.1f\n"+
			"# track format string\n"+
			"titleformat: %s\n"+
			"# album coordinates, hidden\n"+
			"albumx: %.1f\n"+
			"albumy: %.1f\n"+
			"# album format string\n"+
			"albumformat: %s\n"+
			"# reel upper left coordinate\n"+
			"reelx: %.1f\n"+
			"reely: %.1f\n"+
			"# max line width in pixels\n"+
			"titlewidth: %.1f\n"+
			"# zero-padded minutes and zero-padded seconds separated by colon\n"+
			"durationformat: %s\n"+
			"# text color, RGB\n"+
			"textcolor: %s\n",
		cfg.Reel,
		cfg.ArtistX, cfg.ArtistY, cfg.ArtistFormat,
		cfg.TitleX, cfg.TitleY, cfg.TitleFormat,
		cfg.AlbumX, cfg.AlbumY, cfg.AlbumFormat,
		cfg.ReelX, cfg.ReelY,
		cfg.TitleWidth,
		cfg.DurationFormat,
		cfg.TextColor,
	)

	return os.WriteFile(filepath.Join(dir, "config.txt"), []byte(content), 0644)
}

// WriteCassetteTxt writes cassette.txt to the album's music directory.
// This tells Wampy which tape skin to use for that directory.
func WriteCassetteTxt(albumDir, tapeName, reelName string) error {
	content := fmt.Sprintf("tape: %s\nreel: %s\n", tapeName, reelName)
	return os.WriteFile(filepath.Join(albumDir, "cassette.txt"), []byte(content), 0644)
}

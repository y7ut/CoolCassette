package audio

import (
	"os"
	"path/filepath"
	"strings"

	id3 "github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

// AlbumMeta holds the artist and album name extracted from audio tags.
type AlbumMeta struct {
	Artist string
	Album  string
}

// ReadAlbumMeta extracts Artist and Album tags from the given audio file.
// Returns an empty AlbumMeta (not an error) if the tags are missing or unreadable.
func ReadAlbumMeta(filePath string) AlbumMeta {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3", ".wav":
		return readID3Meta(filePath)
	case ".flac":
		return readFLACMeta(filePath)
	case ".m4a", ".m4b", ".aac", ".mp4":
		return readMP4Meta(filePath)
	}
	return AlbumMeta{}
}

func readID3Meta(filePath string) AlbumMeta {
	t, err := id3.Open(filePath, id3.Options{Parse: true})
	if err != nil {
		return AlbumMeta{}
	}
	defer t.Close()
	return AlbumMeta{
		Artist: strings.TrimSpace(t.Artist()),
		Album:  strings.TrimSpace(t.Album()),
	}
}

func readFLACMeta(filePath string) AlbumMeta {
	stream, err := flac.ParseFile(filePath)
	if err != nil {
		return AlbumMeta{}
	}
	defer stream.Close()

	var artist, album string
	for _, block := range stream.Blocks {
		vc, ok := block.Body.(*meta.VorbisComment)
		if !ok {
			continue
		}
		for _, tag := range vc.Tags {
			key := strings.ToUpper(tag[0])
			val := strings.TrimSpace(tag[1])
			switch key {
			case "ARTIST", "ALBUMARTIST":
				if artist == "" {
					artist = val
				}
			case "ALBUM":
				album = val
			}
		}
		break
	}
	return AlbumMeta{Artist: artist, Album: album}
}

func readMP4Meta(filePath string) AlbumMeta {
	f, err := os.Open(filePath)
	if err != nil {
		return AlbumMeta{}
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return AlbumMeta{}
	}
	return AlbumMeta{
		Artist: strings.TrimSpace(m.Artist()),
		Album:  strings.TrimSpace(m.Album()),
	}
}

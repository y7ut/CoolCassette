package audio

import (
	"os"
	"path/filepath"
	"strconv"
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

// TrackMeta holds per-track metadata extracted from an audio file.
type TrackMeta struct {
	Artist      string
	Title       string
	Album       string
	TrackNumber int
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

// ReadTrackMeta extracts per-track metadata (artist, title, album, track number, cover) from the given audio file.
func ReadTrackMeta(filePath string) TrackMeta {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3", ".wav":
		return readID3TrackMeta(filePath)
	case ".flac":
		return readFLACTrackMeta(filePath)
	case ".m4a", ".m4b", ".aac", ".mp4":
		return readMP4TrackMeta(filePath)
	}
	return TrackMeta{}
}

func parseTrackNumber(raw string) int {
	if raw == "" {
		return 0
	}
	parts := strings.SplitN(raw, "/", 2)
	n, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	return n
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

func readID3TrackMeta(filePath string) TrackMeta {
	t, err := id3.Open(filePath, id3.Options{Parse: true})
	if err != nil {
		return TrackMeta{}
	}
	defer t.Close()
	trackNum := 0
	if f := t.GetTextFrame("TRCK"); f.Text != "" {
		trackNum = parseTrackNumber(f.Text)
	}
	return TrackMeta{
		Artist:      strings.TrimSpace(t.Artist()),
		Title:       strings.TrimSpace(t.Title()),
		Album:       strings.TrimSpace(t.Album()),
		TrackNumber: trackNum,
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

func readFLACTrackMeta(filePath string) TrackMeta {
	stream, err := flac.ParseFile(filePath)
	if err != nil {
		return TrackMeta{}
	}
	defer stream.Close()

	var artist, title, album, trackNumRaw string
	for _, block := range stream.Blocks {
		vc, ok := block.Body.(*meta.VorbisComment)
		if !ok {
			continue
		}
		for _, t := range vc.Tags {
			key := strings.ToUpper(t[0])
			val := strings.TrimSpace(t[1])
			switch key {
			case "ARTIST", "ALBUMARTIST":
				if artist == "" {
					artist = val
				}
			case "TITLE":
				title = val
			case "ALBUM":
				album = val
			case "TRACKNUMBER":
				trackNumRaw = val
			}
		}
		break
	}
	return TrackMeta{
		Artist:      artist,
		Title:       title,
		Album:       album,
		TrackNumber: parseTrackNumber(trackNumRaw),
	}
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

func readMP4TrackMeta(filePath string) TrackMeta {
	f, err := os.Open(filePath)
	if err != nil {
		return TrackMeta{}
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return TrackMeta{}
	}
	trackNum, _ := m.Track()
	return TrackMeta{
		Artist:      strings.TrimSpace(m.Artist()),
		Title:       strings.TrimSpace(m.Title()),
		Album:       strings.TrimSpace(m.Album()),
		TrackNumber: trackNum,
	}
}

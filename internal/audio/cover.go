package audio

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	id3 "github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

// CoverImage holds the raw bytes and format of an extracted cover.
type CoverImage struct {
	Data   []byte
	Format string // "jpeg" or "png"
}

// coverImageExts are the supported extensions for cover image files.
var coverImageExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// ExtractCover extracts the cover image from the given audio file.
// If a file named "cover.{jpg,jpeg,png,webp}" exists in the same directory,
// it takes priority over the embedded tag — resized to 400×400 via magick.
// Supports mp3, flac, wav.
func ExtractCover(filePath string) (*CoverImage, error) {
	// Priority: cover image file in album directory
	dir := filepath.Dir(filePath)
	if cover, err := findCoverFile(dir); err == nil {
		return cover, nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return extractMP3Cover(filePath)
	case ".flac":
		return extractFLACCover(filePath)
	case ".wav":
		return extractWAVCover(filePath)
	case ".m4a", ".m4b", ".aac", ".mp4":
		return extractM4ACover(filePath)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

var magickBin = "magick"

func SetMagickPath(p string) { magickBin = p }

// findCoverFile looks for cover.{jpg,jpeg,png,webp} in dir (case-insensitive),
// resizes it to 400×400 via magick, and returns the bytes.
func findCoverFile(dir string) (*CoverImage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if base != "cover" {
			continue
		}
		ext := filepath.Ext(name)
		supported := false
		for _, e := range coverImageExts {
			if ext == e {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		imgPath := filepath.Join(dir, entry.Name())

		cmd := exec.Command(magickBin, imgPath,
			"-resize", "400x400>",
			"-quality", "85",
			"-",
		)
		if data, err := cmd.Output(); err == nil && len(data) > 0 {
			return &CoverImage{Data: data, Format: "jpeg"}, nil
		}

		data, err := os.ReadFile(imgPath)
		if err != nil {
			return nil, fmt.Errorf("cover file: read: %w", err)
		}
		fmt := formatFromExt(ext)
		return &CoverImage{Data: data, Format: fmt}, nil
	}
	return nil, fmt.Errorf("no cover file found in %s", dir)
}

func extractM4ACover(filePath string) (*CoverImage, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open m4a: %w", err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, fmt.Errorf("read m4a tag: %w", err)
	}

	pic := m.Picture()
	if pic == nil {
		return nil, fmt.Errorf("no cover found in %s", filePath)
	}

	return &CoverImage{
		Data:   pic.Data,
		Format: mimeToFormat(pic.MIMEType),
	}, nil
}

func extractMP3Cover(filePath string) (*CoverImage, error) {
	tag, err := id3.Open(filePath, id3.Options{Parse: true})
	if err != nil {
		return nil, fmt.Errorf("open mp3 tag: %w", err)
	}
	defer tag.Close()

	frames := tag.GetFrames(tag.CommonID("Attached picture"))
	for _, f := range frames {
		pic, ok := f.(id3.PictureFrame)
		if !ok {
			continue
		}
		// Prefer front cover (type 3), fall back to any picture
		if pic.PictureType == id3.PTFrontCover || pic.PictureType == id3.PTOther {
			return &CoverImage{
				Data:   pic.Picture,
				Format: mimeToFormat(pic.MimeType),
			}, nil
		}
	}
	// Second pass: accept any picture type
	for _, f := range frames {
		pic, ok := f.(id3.PictureFrame)
		if !ok {
			continue
		}
		return &CoverImage{
			Data:   pic.Picture,
			Format: mimeToFormat(pic.MimeType),
		}, nil
	}
	return nil, fmt.Errorf("no cover found in %s", filePath)
}

func extractFLACCover(filePath string) (*CoverImage, error) {
	stream, err := flac.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("parse flac: %w", err)
	}
	defer stream.Close()

	for _, block := range stream.Blocks {
		if block.Type != meta.TypePicture {
			continue
		}
		pic, ok := block.Body.(*meta.Picture)
		if !ok {
			continue
		}
		return &CoverImage{
			Data:   pic.Data,
			Format: mimeToFormat(pic.MIME),
		}, nil
	}
	return nil, fmt.Errorf("no cover found in %s", filePath)
}

func extractWAVCover(filePath string) (*CoverImage, error) {
	// WAV files may contain ID3 tags (common in modern files)
	// Try reading as ID3 first
	cover, err := extractMP3Cover(filePath)
	if err == nil {
		return cover, nil
	}

	// Fallback: scan for raw JPEG/PNG header bytes in the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read wav: %w", err)
	}

	// Look for JPEG SOI marker
	if idx := bytes.Index(data, []byte{0xFF, 0xD8, 0xFF}); idx >= 0 {
		// Find JPEG EOI marker
		if end := bytes.Index(data[idx:], []byte{0xFF, 0xD9}); end >= 0 {
			jpegData := data[idx : idx+end+2]
			if isValidImage(jpegData) {
				return &CoverImage{Data: jpegData, Format: "jpeg"}, nil
			}
		}
	}

	// Look for PNG header
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if idx := bytes.Index(data, pngHeader); idx >= 0 {
		pngData := data[idx:]
		if isValidImage(pngData) {
			return &CoverImage{Data: pngData, Format: "png"}, nil
		}
	}

	return nil, fmt.Errorf("no cover found in %s", filePath)
}

func isValidImage(data []byte) bool {
	_, _, err := image.Decode(bytes.NewReader(data))
	return err == nil
}

func mimeToFormat(mime string) string {
	mime = strings.ToLower(mime)
	if strings.Contains(mime, "png") {
		return "png"
	}
	return "jpeg"
}

func formatFromExt(ext string) string {
	ext = strings.ToLower(ext)
	if ext == ".png" {
		return "png"
	}
	return "jpeg"
}

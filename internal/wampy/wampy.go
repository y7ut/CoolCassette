package wampy

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CassetteRef describes one cassette.txt assignment.
type CassetteRef struct {
	Tape string `json:"tape"`
	Reel string `json:"reel"`
}

// Validation reports whether a cassette reference points to deployed assets.
type Validation struct {
	TapeDirExists bool `json:"tape_dir_exists"`
	TapeFilesOK   bool `json:"tape_files_ok"`
	ReelDirExists bool `json:"reel_dir_exists"`
	ReelFilesOK   bool `json:"reel_files_ok"`
}

func (v Validation) Built() bool {
	return v.TapeDirExists && v.TapeFilesOK && v.ReelDirExists && v.ReelFilesOK
}

// ReadCassette reads cassette.txt from an album directory.
func ReadCassette(albumDir string) (CassetteRef, error) {
	data, err := os.ReadFile(filepath.Join(albumDir, "cassette.txt"))
	if err != nil {
		return CassetteRef{}, err
	}
	return ParseCassette(string(data)), nil
}

// ParseCassette parses cassette.txt content.
func ParseCassette(content string) CassetteRef {
	var ref CassetteRef
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "tape:"):
			ref.Tape = strings.TrimSpace(strings.TrimPrefix(line, "tape:"))
		case strings.HasPrefix(line, "reel:"):
			ref.Reel = strings.TrimSpace(strings.TrimPrefix(line, "reel:"))
		}
	}
	return ref
}

func TapeDir(wampyDir, tape string) string {
	return filepath.Join(wampyDir, "skins", "cassette", "tape", tape)
}

func ReelDir(wampyDir, reel string) string {
	return filepath.Join(wampyDir, "skins", "cassette", "reel", reel)
}

// ValidateCassetteRef checks whether referenced deployed assets exist.
func ValidateCassetteRef(wampyDir string, ref CassetteRef) Validation {
	result := Validation{}
	if ref.Tape != "" {
		tapeDir := TapeDir(wampyDir, ref.Tape)
		result.TapeDirExists = dirExists(tapeDir)
		result.TapeFilesOK = fileExists(filepath.Join(tapeDir, "tape.pkm")) &&
			fileExists(filepath.Join(tapeDir, "config.txt"))
	}
	if ref.Reel != "" {
		reelDir := ReelDir(wampyDir, ref.Reel)
		result.ReelDirExists = dirExists(reelDir)
		result.ReelFilesOK = fileExists(filepath.Join(reelDir, "atlas.pkm")) &&
			fileExists(filepath.Join(reelDir, "atlas.txt")) &&
			fileExists(filepath.Join(reelDir, "config.txt"))
	}
	return result
}

// ReadKeyValueFile parses simple "key: value" config files.
func ReadKeyValueFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.TrimSpace(strings.ToLower(key))] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// ReadAtlasFrames returns atlas.txt lines as-is.
func ReadAtlasFrames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var frames []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			frames = append(frames, line)
		}
	}
	return frames, scanner.Err()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

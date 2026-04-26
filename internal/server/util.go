package server

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

func albumID(musicDir, albumDir string) string {
	var key string
	if musicDir != "" {
		rel, err := filepath.Rel(musicDir, albumDir)
		if err != nil {
			rel = albumDir
		}
		key = filepath.Clean(rel)
	} else {
		key = filepath.Clean(albumDir)
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

func ResolveEtc1Tool() (string, error) {
	names := []string{"etc1tool"}
	if exeSuffix != "" {
		names = append(names, "etc1tool"+exeSuffix)
	}
	exe, err := os.Executable()
	if err == nil {
		for _, n := range names {
			for _, sub := range []string{"platform-tools", "."} {
				candidate := filepath.Join(filepath.Dir(exe), sub, n)
				if fileExists(candidate) {
					return candidate, nil
				}
			}
		}
	}
	cwd, _ := os.Getwd()
	for _, n := range names {
		for _, sub := range []string{"platform-tools", "."} {
			if fileExists(filepath.Join(cwd, sub, n)) {
				return filepath.Join(cwd, sub, n), nil
			}
		}
	}
	return "", fmt.Errorf("etc1tool not found")
}

func ResolveMagick() string {
	for _, p := range magickCandidates() {
		if fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("magick" + exeSuffix); err == nil {
		return p
	}
	return "magick"
}

func magickCandidates() []string {
	if runtimeOS == "windows" {
		globs, _ := filepath.Glob("C:/Program Files/ImageMagick*/magick.exe")
		result := make([]string, len(globs))
		copy(result, globs)
		return result
	}
	return []string{"/opt/homebrew/bin/magick", "/usr/local/bin/magick"}
}

var exeSuffix string
var runtimeOS string

func init() {
	runtimeOS = runtime.GOOS
	if runtimeOS == "windows" {
		exeSuffix = ".exe"
	}
}

func resolveShell(shell string) string {
	switch shell {
	case "chf", "bhf":
		return shell
	default:
		return "chf"
	}
}

func fallback(value, alt string) string {
	if strings.TrimSpace(value) == "" {
		return alt
	}
	return value
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sanitizeFileComponent(value string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "_")
	return replacer.Replace(value)
}

func fileCreatedAt(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	if sys := info.Sys(); sys != nil {
		v := reflect.ValueOf(sys)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.IsValid() {
			if field := v.FieldByName("Birthtimespec"); field.IsValid() {
				sec := field.FieldByName("Sec")
				nsec := field.FieldByName("Nsec")
				if sec.IsValid() && nsec.IsValid() {
					return time.Unix(sec.Int(), nsec.Int()).UTC()
				}
			}
			if field := v.FieldByName("Birthtime"); field.IsValid() {
				return time.Unix(field.Int(), 0).UTC()
			}
		}
	}
	return info.ModTime().UTC()
}

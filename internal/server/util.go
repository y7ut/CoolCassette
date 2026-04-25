package server

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
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

func resolveEtc1Tool() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "platform-tools", "etc1tool")
		if fileExists(candidate) {
			return candidate, nil
		}
		candidate = filepath.Join(filepath.Dir(exe), "etc1tool")
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "platform-tools", "etc1tool"),
		filepath.Join(cwd, "etc1tool"),
	} {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("etc1tool not found")
}

func resolveShellsDir() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "assets", "templates")
		if dirExists(candidate) {
			return candidate, nil
		}
	}
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "assets", "templates"),
		filepath.Join(cwd, "templates"),
	} {
		if dirExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("shell templates not found")
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

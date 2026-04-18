package deploy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// DeployTape copies the tape skin directory to the wampy skins directory.
//
// Source structure:
//
//	srcDir/
//	  tape.pkm
//	  config.txt
//
// Destination:
//
//	wampyDir/skins/cassette/tape/<slug>/
//	  tape.pkm
//	  config.txt
func DeployTape(srcDir, wampyDir, slug string) error {
	destDir := filepath.Join(wampyDir, "skins", "cassette", "tape", slug)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create wampy tape dir: %w", err)
	}

	files := []string{"tape.pkm", "config.txt"}
	for _, f := range files {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(destDir, f)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", f, err)
		}
	}
	return nil
}

// DeployReel copies the reel atlas directory to the wampy reel skins directory.
//
// Source structure (srcDir):
//
//	atlas.pkm   — ETC1 compressed atlas
//	atlas.txt   — per-frame coordinates
//	config.txt  — animation delay
//
// Destination:
//
//	wampyDir/skins/cassette/reel/<reelName>/
//	  atlas.pkm
//	  atlas.txt
//	  config.txt
func DeployReel(srcDir, wampyDir, reelName string) error {
	destDir := filepath.Join(wampyDir, "skins", "cassette", "reel", reelName)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create wampy reel dir: %w", err)
	}
	for _, f := range []string{"atlas.pkm", "atlas.txt", "config.txt"} {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(destDir, f)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("deploy reel %s: %w", f, err)
		}
	}
	return nil
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

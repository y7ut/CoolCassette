package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove all deployed tape/reel skins and cassette.txt files",
	Long: `Scans the music directory for cassette.txt files, reads the tape and reel
names from each one, removes the corresponding skin directories from the wampy
directory, deletes the cassette.txt from the album directory, and removes any
cached tape.png / reel.png left in album directories.`,
	RunE: runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if len(musicDirs) == 0 {
		return fmt.Errorf("--music-dir is required")
	}
	musicDir := musicDirs[0]
	if wampyDir == "" {
		return fmt.Errorf("--wampy-dir is required")
	}

	entries, err := collectCassetteTxts(musicDir)
	if err != nil {
		return fmt.Errorf("scan music dir: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("Nothing to uninstall.")
		return nil
	}

	fmt.Printf("Found %d installed album(s).\n", len(entries))

	var removed, skipped int
	for _, e := range entries {
		if dryRun {
			fmt.Printf("  [dry-run] would remove tape=%s reel=%s  (%s)\n", e.tape, e.reel, e.albumDir)
			continue
		}

		if verbose {
			fmt.Printf("\n[uninstall] %s\n", e.albumDir)
		}

		// 1. Remove tape skin directory from wampy
		tapeDir := filepath.Join(wampyDir, "skins", "cassette", "tape", e.tape)
		if err := removeDir(tapeDir); err != nil {
			fmt.Fprintf(os.Stderr, "  [warn] remove tape dir %s: %v\n", tapeDir, err)
			skipped++
		} else if verbose {
			fmt.Printf("  removed tape: %s\n", tapeDir)
		}

		// 2. Remove reel skin directory from wampy (only if it was a per-album reel)
		if strings.HasSuffix(e.reel, "_reel") {
			reelDir := filepath.Join(wampyDir, "skins", "cassette", "reel", e.reel)
			if err := removeDir(reelDir); err != nil {
				fmt.Fprintf(os.Stderr, "  [warn] remove reel dir %s: %v\n", reelDir, err)
			} else if verbose {
				fmt.Printf("  removed reel: %s\n", reelDir)
			}
		}

		// 3. Remove cached tape.png and reel.png from album directory
		_ = os.Remove(filepath.Join(e.albumDir, "tape.png"))
		_ = os.Remove(filepath.Join(e.albumDir, "reel.png"))

		// 4. Remove cassette.txt from album directory
		if err := os.Remove(filepath.Join(e.albumDir, "cassette.txt")); err != nil {
			fmt.Fprintf(os.Stderr, "  [warn] remove cassette.txt: %v\n", err)
			skipped++
		} else if verbose {
			fmt.Printf("  removed cassette.txt\n")
		}

		removed++
	}

	if dryRun {
		return nil
	}
	fmt.Printf("\nDone. %d removed, %d skipped.\n", removed, skipped)
	return nil
}

// cassetteTxtEntry holds parsed content of one cassette.txt.
type cassetteTxtEntry struct {
	albumDir string
	tape     string
	reel     string
}

// collectCassetteTxts walks musicDir and returns all directories
// that contain a valid cassette.txt.
func collectCassetteTxts(musicDir string) ([]cassetteTxtEntry, error) {
	var result []cassetteTxtEntry

	err := filepath.WalkDir(musicDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		cassettePath := filepath.Join(path, "cassette.txt")
		data, err := os.ReadFile(cassettePath)
		if err != nil {
			return nil // no cassette.txt here
		}

		tape, reel := parseCassetteTxt(string(data))
		if tape == "" {
			return nil // invalid / empty
		}

		result = append(result, cassetteTxtEntry{
			albumDir: path,
			tape:     tape,
			reel:     reel,
		})
		return filepath.SkipDir // don't descend further
	})

	return result, err
}

// parseCassetteTxt extracts tape and reel values from cassette.txt content.
func parseCassetteTxt(content string) (tape, reel string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "tape:") {
			tape = strings.TrimSpace(strings.TrimPrefix(line, "tape:"))
		} else if strings.HasPrefix(line, "reel:") {
			reel = strings.TrimSpace(strings.TrimPrefix(line, "reel:"))
		}
	}
	return
}

// removeDir removes a directory and all its contents, ignoring not-found errors.
func removeDir(path string) error {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

# CoolCassette

[English](README.md) | [中文](README_CN.md)

AI-powered cassette tape skin generator for the [Wampy](https://github.com/thedannicraft/wampy) music player plugin on Sony NW-series Walkman devices.

Scans your music library, extracts album artwork, generates custom cassette tape visuals using AI, and deploys them directly to your device — including animated reel sprites.

---

## What it can do

- **Automatic cover extraction**: Extract album artwork from audio file tags (MP3, FLAC, WAV, M4A) or cover images in the album directory 
- **tape skins with dynamic reel animation**: Create unique cassette skins based on album covers, including matching tape shells and generate rotation animations
- **One-click device deployment**: Compress and deploy generated skins and animations directly to the wampy skins directory on your Walkman
- **Automatic album association**: Generate configuration files in each album directory so Wampy automatically matches the correct tape skin to each album

---

## Requirements

- Go 1.21+
- [ImageMagick 7](https://imagemagick.org) (`magick` command)
- Android Platform Tools `etc1tool` — for PKM compression  
  Download: [Mac](https://dl.google.com/android/repository/platform-tools-latest-darwin.zip) · [Windows](https://dl.google.com/android/repository/platform-tools-latest-windows.zip) · [Linux](https://dl.google.com/android/repository/platform-tools-latest-linux.zip)  
  Place `etc1tool` in `platform-tools/` next to the binary or in `PATH`
- An [OpenRouter](https://openrouter.ai) API key (or OpenAI)

---

## Installation

```bash
git clone https://github.com/coolcassette/coolcassette
cd coolcassette
go build -o coolcassette .
```

Set your API key:

```bash
export OPENROUTER_API_KEY=sk-or-...
# or
export OPENAI_API_KEY=sk-...
```

---

## Commands

### `preview`

Generate a tape preview for a single album directory. Saves `tape.png` and `reel.png` alongside the music files for inspection before committing to a full generate run.

```bash
coolcassette preview ~/Music/Nujabes/Modal\ Soul \
  --api-key $OPENROUTER_API_KEY
```

The cached `tape.png` is reused by `generate`, skipping the API call.

---

### `generate`

Scan a music directory and generate + deploy skins for all unprocessed albums.

```bash
coolcassette generate \
  --music-dir /Volumes/WALKMAN/MUSIC \
  --wampy-dir /Volumes/WALKMAN/wampy \
  --api-key $OPENROUTER_API_KEY
```

Each album gets:
- `wampy/skins/cassette/tape/<slug>_tape/` — tape skin (PKM + config)
- `wampy/skins/cassette/reel/<slug>_reel/` — reel atlas (PKM + atlas.txt + config)
- `<album-dir>/cassette.txt` — skin assignment for Wampy

Albums with an existing valid `cassette.txt` are skipped (already processed). Use `--force` to regenerate.

**Cover image priority:**
1. `cover.{jpg,jpeg,png,webp}` in the album directory (resized to 400×400)
2. Embedded cover art from audio file tags
3. API call to generate from scratch

---

### `share`

Build skins into a portable directory without deploying to a device. Produces a self-contained `preview.html` with the tape animation embedded (no external files needed).

```bash
coolcassette share \
  --music-dir ~/Music/Nujabes \
  --api-key $OPENROUTER_API_KEY \
  --output-dir ./share
```

Output structure:

```
share/
  <Artist>/
    <Album>/
      tape/<slug>_tape/
        tape.pkm
        config.txt
      reel/<slug>_reel/
        atlas.pkm
        atlas.txt
        config.txt
      cassette.txt
      preview.html     ← self-contained tape animation, open in browser
```

---

### `uninstall`

Remove all deployed skins and reset album directories to their original state.

```bash
coolcassette uninstall \
  --music-dir /Volumes/WALKMAN/MUSIC \
  --wampy-dir /Volumes/WALKMAN/wampy
```

Reads each `cassette.txt`, removes the corresponding tape/reel directories from wampy, deletes cached `tape.png`/`reel.png`, and removes `cassette.txt` from the album directory.

Use `--dry-run` to preview what would be removed.

---

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--music-dir` | — | Path to music root directory |
| `--wampy-dir` | — | Path to wampy directory on device |
| `--api-key` | env | API key (`OPENROUTER_API_KEY` or `OPENAI_API_KEY`) |
| `--provider` | `openrouter` | `openrouter` or `openai` |
| `--shell` | `random` | Shell template: `chf`, `bhf`, or `random` |
| `--reel` | `other` | Fallback reel name if per-album reel fails |
| `--force` | false | Reprocess albums that already have `cassette.txt` |
| `--dry-run` | false | Print plan without writing any files |
| `--verbose` | false | Verbose output |

---

## Skin naming

To avoid collisions between tape and reel directories, skins are named using audio tag metadata:

- Slug format: `<artist>_<album>` (sanitized, lowercase)
- Tape directory: `<slug>_tape`
- Reel directory: `<slug>_reel`

If tags are missing, the album directory path is used as fallback.

---

## Reel animation

The reel sprite is generated directly from the tape image:

- Template region: 440×110 px at position (180, 161) on the tape
- Two circles extracted: left center (57, 56), right center (383, 56), radius 42
- 40 frames × 9° rotation = full 360°
- Frame delay: 55ms (Wampy default)
- Atlas layout: 4 columns × 10 rows → 1760×1100 px PNG → ETC1 PKM

---

## Supported audio formats

MP3, FLAC, WAV, M4A, M4B, AAC, MP4

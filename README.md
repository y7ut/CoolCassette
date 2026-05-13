# CoolCassette

[English](README.md) | [中文](README_CN.md)

Turn your music library into custom cassette tape skins for the [Wampy](https://github.com/thedannicraft/wampy) player on Sony NW-series Walkman.

CoolCassette reads your album covers, uses AI to generate unique cassette visuals (shell + label + animated reels), and deploys them straight to your device.

---

## Features

- Scans your music library and extracts album art from file tags or cover images
- Generates full 800×480 cassette tape skins using AI image generation
- Creates animated reel sprites (40-frame rotation loop) for each tape
- Deploys directly to Walkman via `wampy/skins/` directory structure
- Desktop app with album browser, audio preview, and one-click skin generation
- CLI for batch processing and automation

---

## Quick Start

### Desktop App (macOS)

1. Download `CoolCassette-<version>-macos-arm64.tar.gz` from [Releases](../../releases)
2. Extract and move `CoolCassette.app` to Applications
3. Open the app, configure your music directory in Settings
4. Click any album → Generate Preview → Publish

### CLI

```bash
# Install
git clone https://github.com/coolcassette/coolcassette
cd coolcassette && go build -o coolcassette .

# Configure API key
echo '{"api_key":"your-key","provider":"custom","base_url":"https://openrouter.ai/api/v1","model":"google/gemini-3.1-flash-image-preview"}' > ~/.coolcassette.json
# or: export CUSTOM_API_KEY=your-key
#     export CUSTOM_BASE_URL=https://openrouter.ai/api/v1
#     export CUSTOM_MODEL=google/gemini-3.1-flash-image-preview

# Preview one album
coolcassette preview ~/Music/AlbumName

# Generate & deploy all
coolcassette generate --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy
```

---

## Requirements

| Tool | Why | Install |
|------|-----|---------|
| ImageMagick 7 | Cover resize, tape compositing, reel atlas | `brew install imagemagick` |

AI provider account (one of):
- Any OpenAI-compatible API (OpenRouter, SiliconFlow, etc.) — custom provider
- [Google AI](https://ai.google.dev) (Gemini) — direct provider

> `etc1tool` (Android Platform Tools) is bundled in the release packages. CLI users can download it from [Android Developer](https://developer.android.com/tools/releases/platform-tools) and place it in `platform-tools/` next to the binary.

---

## Commands

### `preview` — Generate and preview a single album

```bash
coolcassette preview ~/Music/Artist/Album
```

Creates `tape.png` and `reel.png` in the album directory. The cached files are reused by `generate`, skipping the API call.

### `generate` — Batch generate and deploy

```bash
coolcassette generate --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy
```

Scans all albums, generates skins for unprocessed ones, and deploys to device. Already-processed albums (with `cassette.txt`) are skipped. Use `--force` to regenerate.

### `share` — Export portable skins

```bash
coolcassette share --music-dir ~/Music --output-dir ./share
```

Generates skins into a local folder with self-contained `preview.html` files. No device needed.

### `server` — Start API server

```bash
coolcassette server --listen 127.0.0.1:7350
```

Starts an HTTP API server for the desktop app or custom integrations.

### `uninstall` — Remove deployed skins

```bash
coolcassette uninstall --music-dir ~/Music --wampy-dir /Volumes/WALKMAN/wampy --dry-run
```

Removes all skins and resets `cassette.txt` files. Use `--dry-run` to preview.

---

## Configuration

Config file is discovered in this order:

1. **`~/.coolcassette.json`** — user home directory (highest priority)
2. **`coolcassette.json`** — next to the executable (bundled with release packages)

Configuration priority: **CLI flags > environment variables > config file > defaults**

### `~/.coolcassette.json`

```json
{
  "api_key": "your-api-key",
  "provider": "custom",
  "base_url": "https://openrouter.ai/api/v1",
  "model": "google/gemini-3.1-flash-image-preview"
}
```

Configuration priority: **CLI flags > environment variables > user home config > executable directory config > defaults**

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--api-key` | `CUSTOM_API_KEY`, `API_KEY` | config file | AI provider API key |
| `--provider` | `PROVIDER` | `custom` | `custom` or `google` |
| `--base-url` | `CUSTOM_BASE_URL` | config file | OpenAI-compatible API base URL |
| `--model` | `CUSTOM_MODEL` | config file | Model name for image generation |
| `--music-dir` | — | — | Music library path (repeatable) |
| `--wampy-dir` | — | — | Wampy directory on device |
| `--shell` | — | `random` | Shell template: `chf`, `bhf`, `random` |
| `--force` | — | false | Regenerate existing skins |
| `--verbose` | — | false | Verbose output |

---

## Supported Formats

MP3, FLAC, WAV, M4A, M4B, AAC, MP4

Cover art priority: `cover.jpg/png` in album dir → embedded tag → AI generation

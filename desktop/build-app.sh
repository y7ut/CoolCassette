#!/bin/bash
set -e
cd "$(dirname "$0")"

if ! command -v magick &>/dev/null; then
    echo "ERROR: magick (ImageMagick) not found in PATH"
    echo "  brew install imagemagick"
    exit 1
fi

echo "==> Running wails build..."
wails build -skipbindings -tags "desktop,production"

APP_BUNDLE="build/bin/CoolCassette.app"
MACOS_DIR="$APP_BUNDLE/Contents/MacOS"
ROOT="$(cd .. && pwd)"

echo "==> Copying platform-tools and assets..."
cp -R "$ROOT/platform-tools" "$MACOS_DIR/platform-tools"
cp -R "$ROOT/assets" "$MACOS_DIR/assets"

echo "==> Done: $APP_BUNDLE"
ls -lh "$MACOS_DIR/CoolCassette"

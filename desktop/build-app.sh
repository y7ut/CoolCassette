#!/bin/bash
set -e
cd "$(dirname "$0")"

if [ "${SKIP_MAGICK_CHECK:-}" != "1" ]; then
    if ! command -v magick &>/dev/null; then
        echo "ERROR: magick (ImageMagick) not found in PATH"
        echo "  brew install imagemagick"
        exit 1
    fi
fi

APP_BUNDLE="build/bin/CoolCassette.app"
MACOS_DIR="$APP_BUNDLE/Contents/MacOS"
ROOT="$(cd .. && pwd)"

echo "==> Building frontend..."
cd "$ROOT/web" && npm run build
cd -

echo "==> Syncing frontend_dist..."
rm -rf frontend_dist
cp -R "$ROOT/web/dist" frontend_dist

echo "==> Building app..."
wails build -tags "desktop,production" -s

echo "==> Copying platform-tools..."
cp -R "$ROOT/platform-tools" "$MACOS_DIR/platform-tools"
cp -R "$ROOT/assets" "$MACOS_DIR/assets"

echo "==> Done: $APP_BUNDLE"
ls -lh "$MACOS_DIR/CoolCassette"

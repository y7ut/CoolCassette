#!/bin/bash
set -e
cd "$(dirname "$0")"
ROOT="$(cd .. && pwd)"
OUTDIR="build/dist-windows"
NAME="CoolCassette"
VERSION="${1:-dev}"

echo "==> Building frontend..."
cd "$ROOT/web"
npm run build 2>&1 | tail -3

echo "==> Copying frontend_dist..."
cd "$ROOT/desktop"
rm -rf frontend_dist
cp -R "$ROOT/web/dist" frontend_dist

echo "==> Cross-compiling for windows/amd64..."
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
  wails build -s -tags "desktop,production" -platform windows/amd64 -o "${NAME}.exe" 2>&1 | tail -5

echo "==> Assembling zip package..."
rm -rf "$OUTDIR"
mkdir -p "$OUTDIR/$NAME"

cp "build/bin/${NAME}.exe" "$OUTDIR/$NAME/"

if [ -d "$ROOT/platform-tools-win" ]; then
    cp -R "$ROOT/platform-tools-win" "$OUTDIR/$NAME/platform-tools"
    echo "    included Windows platform-tools"
else
    echo "    WARNING: platform-tools-win/ not found — etc1tool will not work"
    echo "    Download from https://dl.google.com/android/repository/platform-tools-latest-windows.zip"
    echo "    and extract to $ROOT/platform-tools-win/"
fi

cp -R "$ROOT/assets" "$OUTDIR/$NAME/assets"

cp "coolcassette.json.example" "$OUTDIR/$NAME/coolcassette.json"

ZIPNAME="${NAME}-${VERSION}-windows-amd64.zip"
cd "$OUTDIR"
rm -f "$ROOT/desktop/$ZIPNAME"
zip -r "$ROOT/desktop/$ZIPNAME" "$NAME"
cd ..

echo "==> Done: $ZIPNAME"
ls -lh "$ZIPNAME"

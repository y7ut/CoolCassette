#!/bin/bash
set -e
cd "$(dirname "$0")/.."

echo "==> Ensuring frontend_dist for embed..."
rm -rf desktop/frontend_dist
cp -R web/dist desktop/frontend_dist

echo "==> Starting wails dev..."
cd desktop
wails dev -m

#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [[ ! -d "$ROOT_DIR/frontend/dist" ]]; then
  echo "frontend/dist not found. Run: npm --prefix frontend run build"
  exit 1
fi

mkdir -p "$ROOT_DIR/backend/frontend"
rm -rf "$ROOT_DIR/backend/frontend/dist"
cp -R "$ROOT_DIR/frontend/dist" "$ROOT_DIR/backend/frontend/"
cp "$ROOT_DIR/logo.svg" "$ROOT_DIR/backend/logo.svg"

echo "Backend embedded assets synchronized."

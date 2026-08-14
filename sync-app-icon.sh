#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_ICON="$SCRIPT_DIR/assets/appicon.png"
OUTPUT_ICON="$SCRIPT_DIR/build/appicon.png"

if [[ ! -f "$SOURCE_ICON" ]]; then
  echo "找不到來源 icon：$SOURCE_ICON"
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_ICON")"
cp "$SOURCE_ICON" "$OUTPUT_ICON"
echo "已同步 App Icon：$OUTPUT_ICON"

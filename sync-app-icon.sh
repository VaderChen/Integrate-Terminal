#!/bin/zsh

set -euo pipefail

cd "$(dirname "$0")"

SOURCE_ICON="./assets/appicon.png"
OUTPUT_ICON="./build/appicon.png"

if [[ ! -f "$SOURCE_ICON" ]]; then
  echo "找不到來源 icon：$SOURCE_ICON"
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_ICON")"
cp "$SOURCE_ICON" "$OUTPUT_ICON"
echo "已同步 App Icon：$OUTPUT_ICON"

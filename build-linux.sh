#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_PATH="$SCRIPT_DIR/build/bin/IntegTERM"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "此腳本只能在 Linux 上執行。"
  exit 1
fi

if [[ "$(uname -m)" != "x86_64" ]]; then
  echo "此腳本只支援 Linux x64（x86_64）。"
  exit 1
fi

for command_name in go node npm pkg-config; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "缺少必要指令：$command_name"
    exit 1
  fi
done

if ! pkg-config --exists gtk+-3.0; then
  echo "缺少必要開發套件：gtk+-3.0"
  exit 1
fi

build_tags=()
if pkg-config --exists webkit2gtk-4.1 libsoup-3.0; then
  build_tags+=(webkit2_41)
elif ! pkg-config --exists webkit2gtk-4.0 libsoup-2.4; then
  echo "缺少 WebKitGTK 開發套件：需要 webkit2gtk-4.1 與 libsoup-3.0，或 webkit2gtk-4.0 與 libsoup-2.4。"
  exit 1
fi

if pkg-config --exists ayatana-appindicator3-0.1; then
  :
elif pkg-config --exists appindicator3-0.1; then
  build_tags+=(legacy_appindicator)
else
  echo "缺少系統列開發套件：需要 ayatana-appindicator3-0.1 或 appindicator3-0.1。"
  exit 1
fi

cd "$SCRIPT_DIR"
WAILS_VERSION="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"
APP_VERSION="$(node -p "require('./wails.json').info.productVersion")"

if [[ -z "$WAILS_VERSION" || "$WAILS_VERSION" == "<no value>" ]]; then
  echo "無法取得 Wails 版本。"
  exit 1
fi

export CGO_ENABLED=1
export VITE_APP_VERSION="$APP_VERSION"

build_arguments=(build -clean -nopackage -platform linux/amd64)
if (( ${#build_tags[@]} > 0 )); then
  build_arguments+=(-tags "${build_tags[*]}")
fi

echo "建置 Linux x64 執行檔..."
go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" "${build_arguments[@]}"

if [[ ! -x "$OUTPUT_PATH" ]]; then
  echo "建置失敗：找不到 $OUTPUT_PATH"
  exit 1
fi

echo "完成：$OUTPUT_PATH"

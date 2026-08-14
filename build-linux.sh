#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_PATH="$SCRIPT_DIR/build/bin/IntegTERM"
LICENSE_OUTPUT_DIR="$SCRIPT_DIR/build/bin/licenses"
METADATA_OUTPUT_PATH="$SCRIPT_DIR/build/bin/build-metadata.json"
BUILD_SOURCE_URL="${BUILD_SOURCE_URL:-https://github.com/VaderChen/Integrate-Terminal}"

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

echo "產生第三方授權清冊..."
node "$SCRIPT_DIR/scripts/generate-third-party-notices.mjs"

if [[ -z "${BUILD_COMMIT:-}" || -z "${BUILD_TAG:-}" || -z "${BUILD_STATE:-}" ]]; then
  if command -v git >/dev/null 2>&1 && git -C "$SCRIPT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    BUILD_COMMIT="${BUILD_COMMIT:-$(git -C "$SCRIPT_DIR" rev-parse HEAD)}"
    exact_tag="$(git -C "$SCRIPT_DIR" describe --tags --exact-match HEAD 2>/dev/null || true)"
    BUILD_TAG="${BUILD_TAG:-${exact_tag:-untagged}}"
    if [[ -z "${BUILD_STATE:-}" ]]; then
      if [[ -n "$(git -C "$SCRIPT_DIR" status --porcelain=v1 --untracked-files=normal)" ]]; then
        BUILD_STATE="dirty"
      else
        BUILD_STATE="clean"
      fi
    fi
  else
    BUILD_COMMIT="${BUILD_COMMIT:-unknown}"
    BUILD_TAG="${BUILD_TAG:-untagged}"
    BUILD_STATE="${BUILD_STATE:-unknown}"
  fi
fi

for metadata_value in "$BUILD_COMMIT" "$BUILD_TAG" "$BUILD_STATE" "$BUILD_SOURCE_URL"; do
  if [[ ! "$metadata_value" =~ ^[A-Za-z0-9._/:+-]+$ ]]; then
    echo "建置中繼資料包含不支援的字元：$metadata_value"
    exit 1
  fi
done

BUILD_LDFLAGS="-X github.com/VaderChen/Integrate-Terminal/internal/version.Commit=$BUILD_COMMIT -X github.com/VaderChen/Integrate-Terminal/internal/version.Tag=$BUILD_TAG -X github.com/VaderChen/Integrate-Terminal/internal/version.BuildState=$BUILD_STATE -X github.com/VaderChen/Integrate-Terminal/internal/version.SourceURL=$BUILD_SOURCE_URL"

build_arguments=(build -clean -nopackage -platform linux/amd64 -ldflags "$BUILD_LDFLAGS")
if (( ${#build_tags[@]} > 0 )); then
  build_arguments+=(-tags "${build_tags[*]}")
fi

echo "建置 Linux x64 執行檔..."
go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" "${build_arguments[@]}"

if [[ ! -x "$OUTPUT_PATH" ]]; then
  echo "建置失敗：找不到 $OUTPUT_PATH"
  exit 1
fi

rm -rf "$LICENSE_OUTPUT_DIR"
mkdir -p "$LICENSE_OUTPUT_DIR"
cp "$SCRIPT_DIR/LICENSE" "$LICENSE_OUTPUT_DIR/GPL-3.0.txt"
cp "$SCRIPT_DIR/THIRD-PARTY-NOTICES.md" "$LICENSE_OUTPUT_DIR/THIRD-PARTY-NOTICES.md"
cp "$SCRIPT_DIR/THIRD-PARTY-LICENSES.txt" "$LICENSE_OUTPUT_DIR/THIRD-PARTY-LICENSES.txt"
node "$SCRIPT_DIR/scripts/write-build-metadata.mjs" \
  "$METADATA_OUTPUT_PATH" \
  "$APP_VERSION" \
  "$BUILD_COMMIT" \
  "$BUILD_TAG" \
  "$BUILD_STATE" \
  "$BUILD_SOURCE_URL"

echo "完成：$OUTPUT_PATH"
echo "授權文件：$LICENSE_OUTPUT_DIR"
echo "來源版本：$BUILD_TAG ($BUILD_COMMIT, $BUILD_STATE)"

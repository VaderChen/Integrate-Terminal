#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

BUILD_OUTPUT_PATH="./build/bin/IntegTERM"
OUTPUT_PATH="./dist/IntegTERM"
LICENSE_OUTPUT_DIR="./dist/licenses"
METADATA_OUTPUT_PATH="./dist/build-metadata.json"
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

mkdir -p "./dist"
WAILS_VERSION="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"

BUILD_VERSION_JSON="$(node "./scripts/resolve-build-version.mjs" --sync)"
build_version_field() {
  node -e 'const value = JSON.parse(process.argv[1])[process.argv[2]]; process.stdout.write(String(value));' "$BUILD_VERSION_JSON" "$1"
}
APP_VERSION="$(build_version_field marketingVersion)"
APP_BUILD_LABEL="$(build_version_field buildLabel)"
APP_DISPLAY_VERSION="$(build_version_field displayVersion)"
APP_BUNDLE_VERSION="$(build_version_field bundleVersion)"
BUILD_TIME_SOURCE="$(build_version_field timeSource)"

if [[ -z "$WAILS_VERSION" || "$WAILS_VERSION" == "<no value>" ]]; then
  echo "無法取得 Wails 版本。"
  exit 1
fi

export CGO_ENABLED=1
export VITE_APP_VERSION="$APP_DISPLAY_VERSION"
export APP_MARKETING_VERSION="$APP_VERSION"
export APP_BUILD_LABEL
export APP_BUNDLE_VERSION

echo "產生第三方授權清冊..."
node "./scripts/generate-third-party-notices.mjs"
echo "版本號：$APP_DISPLAY_VERSION"
echo "時間來源：$BUILD_TIME_SOURCE"

if [[ -z "${BUILD_COMMIT:-}" || -z "${BUILD_TAG:-}" || -z "${BUILD_STATE:-}" ]]; then
  if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse HEAD)}"
    exact_tag="$(git describe --tags --exact-match HEAD 2>/dev/null || true)"
    BUILD_TAG="${BUILD_TAG:-${exact_tag:-untagged}}"
    if [[ -z "${BUILD_STATE:-}" ]]; then
      if [[ -n "$(git status --porcelain=v1 --untracked-files=normal)" ]]; then
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

BUILD_LDFLAGS="-X github.com/VaderChen/Integrate-Terminal/internal/version.Product=$APP_VERSION -X github.com/VaderChen/Integrate-Terminal/internal/version.Commit=$BUILD_COMMIT -X github.com/VaderChen/Integrate-Terminal/internal/version.Tag=$BUILD_TAG -X github.com/VaderChen/Integrate-Terminal/internal/version.BuildState=$BUILD_STATE -X github.com/VaderChen/Integrate-Terminal/internal/version.SourceURL=$BUILD_SOURCE_URL"

build_arguments=(build -clean -nopackage -skipembedcreate -platform linux/amd64 -ldflags "$BUILD_LDFLAGS")
if (( ${#build_tags[@]} > 0 )); then
  build_arguments+=(-tags "${build_tags[*]}")
fi

echo "建置 Linux x64 執行檔..."
go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" "${build_arguments[@]}"

if [[ ! -x "$BUILD_OUTPUT_PATH" ]]; then
  echo "建置失敗：找不到 $BUILD_OUTPUT_PATH"
  exit 1
fi

rm -rf "$OUTPUT_PATH" "$LICENSE_OUTPUT_DIR"
mv "$BUILD_OUTPUT_PATH" "$OUTPUT_PATH"
mkdir -p "$LICENSE_OUTPUT_DIR"
cp "./LICENSE" "$LICENSE_OUTPUT_DIR/GPL-3.0.txt"
cp "./THIRD-PARTY-NOTICES.md" "$LICENSE_OUTPUT_DIR/THIRD-PARTY-NOTICES.md"
cp "./THIRD-PARTY-LICENSES.txt" "$LICENSE_OUTPUT_DIR/THIRD-PARTY-LICENSES.txt"
node "./scripts/write-build-metadata.mjs" \
  "$METADATA_OUTPUT_PATH" \
  "$APP_VERSION" \
  "$BUILD_COMMIT" \
  "$BUILD_TAG" \
  "$BUILD_STATE" \
  "$BUILD_SOURCE_URL"

echo "完成：$OUTPUT_PATH"
echo "授權文件：$LICENSE_OUTPUT_DIR"
echo "來源版本：$BUILD_TAG ($BUILD_COMMIT, $BUILD_STATE)"

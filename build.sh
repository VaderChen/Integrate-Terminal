#!/bin/zsh

set -euo pipefail

cd "$(dirname "$0")"

FRONTEND_DIR="./frontend"
APP_NAME="IntegTERM"
DIST_DIR="./dist"
APP_PATH="$DIST_DIR/$APP_NAME.app"
TMP_ROOT=""
export MACOSX_DEPLOYMENT_TARGET="12.0"
export CGO_CFLAGS="-mmacosx-version-min=12.0"
export CGO_LDFLAGS="-mmacosx-version-min=12.0"
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1
CODESIGN_IDENTITY="${CODESIGN_IDENTITY:--}"
APP_BUNDLE_ID="${APP_BUNDLE_ID:-com.vader.integterm}"
BUILD_SOURCE_URL="${BUILD_SOURCE_URL:-https://github.com/VaderChen/Integrate-Terminal}"

if (( $# > 0 )); then
  echo "用法：$0"
  echo "公開版使用 ad-hoc 簽章，不啟用 App Sandbox。"
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" || "$(uname -m)" != "arm64" ]]; then
  echo "此腳本只支援 Apple Silicon macOS。"
  exit 1
fi

cleanup_tmp_root() {
  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" ]]; then
    rm -rf "$TMP_ROOT"
  fi
}

trap cleanup_tmp_root EXIT

if [[ -f "$HOME/.zshrc" ]]; then
  source "$HOME/.zshrc"
fi

APP_MARKETING_VERSION="${APP_MARKETING_VERSION:-1.$(date +%y).$(date +%m%d)}"
APP_BUILD_LABEL="${APP_BUILD_LABEL:-$(date +%H%M)}"
APP_DISPLAY_VERSION="$APP_MARKETING_VERSION build $APP_BUILD_LABEL"
APP_BUNDLE_VERSION="${APP_BUNDLE_VERSION:-${APP_MARKETING_VERSION}${APP_BUILD_LABEL}}"

if [[ ! "$APP_MARKETING_VERSION" =~ '^[0-9]+([.][0-9]+)*$' ]]; then
  echo "APP_MARKETING_VERSION 格式錯誤：$APP_MARKETING_VERSION"
  exit 1
fi

if [[ ! "$APP_BUNDLE_VERSION" =~ '^[0-9]+([.][0-9]+)*$' ]]; then
  echo "APP_BUNDLE_VERSION 格式錯誤：$APP_BUNDLE_VERSION"
  exit 1
fi

if [[ ! "$APP_BUNDLE_ID" =~ '^[A-Za-z0-9-]+([.][A-Za-z0-9-]+)+$' ]]; then
  echo "APP_BUNDLE_ID 格式錯誤：$APP_BUNDLE_ID"
  exit 1
fi

export VITE_APP_VERSION="$APP_DISPLAY_VERSION"
export APP_MARKETING_VERSION
export APP_BUNDLE_VERSION

cleanup_appledouble() {
  local target_path="$1"
  if [[ -e "$target_path" ]]; then
    find "$target_path" -name '._*' -print -delete 2>/dev/null || true
    find "$target_path" -name '.DS_Store' -print -delete 2>/dev/null || true
  fi
}

cleanup_codesign_artifacts() {
  local target_path="$1"
  if [[ -e "$target_path" ]]; then
    find "$target_path" -name '_CodeSignature' -type d -prune -exec rm -rf {} + 2>/dev/null || true
    find "$target_path" -name 'CodeResources' -type f -delete 2>/dev/null || true
  fi
}

normalize_bundle_permissions() {
  local target_path="$1"
  if [[ -e "$target_path" ]]; then
    chmod -R u+rwX,go+rX "$target_path" 2>/dev/null || true
  fi
}

required_commands=(go node npm rsync codesign ditto)
for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少必要指令: $cmd"
    echo "請先安裝完成後再執行 $0"
    exit 1
  fi
done

WAILS_VERSION="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2 2>/dev/null || true)"
if [[ -z "$WAILS_VERSION" || "$WAILS_VERSION" == "<no value>" ]]; then
  echo "無法取得 go.mod 指定的 Wails 版本。"
  exit 1
fi
WAILS_COMMAND=(go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION")

cd "$FRONTEND_DIR"
if [[ ! -d node_modules ]]; then
  echo "安裝前端依賴中..."
  npm install
fi

cd ..
echo "整理 Go 模組..."
go mod tidy

echo "產生第三方授權清冊..."
node "./scripts/generate-third-party-notices.mjs"

echo "同步 App Icon..."
"./sync-app-icon.sh"

echo "同步產品版本..."
node <<'EOF'
const fs = require('fs');
const path = require('path');

const configPath = path.join(process.cwd(), 'wails.json');
const serviceVersionPath = path.join(process.cwd(), 'internal', 'version', 'version.json');
const appVersion = process.env.APP_MARKETING_VERSION || '1.00.00';
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));

config.info = {
  ...(config.info || {}),
  productVersion: appVersion,
};

fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
fs.writeFileSync(serviceVersionPath, `${JSON.stringify({ productVersion: appVersion }, null, 2)}\n`);
EOF

echo "建置前端資產..."
echo "版本號: $APP_DISPLAY_VERSION"
cd "$FRONTEND_DIR"
rm -rf dist
npm run build
cleanup_appledouble "$FRONTEND_DIR/dist"

cd ..
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
  if [[ ! "$metadata_value" =~ '^[A-Za-z0-9._/:+-]+$' ]]; then
    echo "建置中繼資料包含不支援的字元：$metadata_value"
    exit 1
  fi
done

BUILD_LDFLAGS="-X github.com/VaderChen/Integrate-Terminal/internal/version.Product=$APP_MARKETING_VERSION -X github.com/VaderChen/Integrate-Terminal/internal/version.Commit=$BUILD_COMMIT -X github.com/VaderChen/Integrate-Terminal/internal/version.Tag=$BUILD_TAG -X github.com/VaderChen/Integrate-Terminal/internal/version.BuildState=$BUILD_STATE -X github.com/VaderChen/Integrate-Terminal/internal/version.SourceURL=$BUILD_SOURCE_URL"

mkdir -p "$DIST_DIR" "./.codex-tmp"
rm -rf "$APP_PATH"
TMP_ROOT="$(mktemp -d "./.codex-tmp/integterm-build.XXXXXX")"
STAGING_DIR="$TMP_ROOT/project"
STAGING_APP_PATH="$STAGING_DIR/build/bin/$APP_NAME.app"

echo "同步專案到本機暫存目錄..."
mkdir -p "$STAGING_DIR"
rsync -a \
  --exclude '/.git/' \
  --exclude '/.codex-tmp/' \
  --exclude '.DS_Store' \
  --exclude '._*' \
  --exclude '*.bak' \
  --exclude '/cert/' \
  --exclude '/data/' \
  --exclude '/dist/' \
  --exclude '/build/bin/' \
  --exclude '/frontend/node_modules/.cache/' \
  ./ "$STAGING_DIR/"

cleanup_appledouble "$STAGING_DIR"
cleanup_appledouble "$STAGING_DIR/frontend/dist"

if [[ ! -s "$STAGING_DIR/frontend/dist/index.html" ]]; then
  echo "建置失敗：暫存專案缺少 frontend/dist/index.html，無法嵌入 Wails 前端資產。"
  exit 1
fi

echo "開始打包 Wails 應用程式..."
(
  cd "$STAGING_DIR"
  "${WAILS_COMMAND[@]}" build -clean -s -skipembedcreate -ldflags "$BUILD_LDFLAGS"
)

if [[ ! -d "$STAGING_APP_PATH" ]]; then
  echo "建置失敗：找不到暫存產物 $STAGING_APP_PATH"
  exit 1
fi

/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $APP_BUNDLE_VERSION" "$STAGING_APP_PATH/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier $APP_BUNDLE_ID" "$STAGING_APP_PATH/Contents/Info.plist"
APP_LICENSE_DIR="$STAGING_APP_PATH/Contents/Resources/Licenses"
mkdir -p "$APP_LICENSE_DIR"
cp "$STAGING_DIR/LICENSE" "$APP_LICENSE_DIR/GPL-3.0.txt"
cp "$STAGING_DIR/THIRD-PARTY-NOTICES.md" "$APP_LICENSE_DIR/THIRD-PARTY-NOTICES.md"
cp "$STAGING_DIR/THIRD-PARTY-LICENSES.txt" "$APP_LICENSE_DIR/THIRD-PARTY-LICENSES.txt"
node "$STAGING_DIR/scripts/write-build-metadata.mjs" \
  "$STAGING_APP_PATH/Contents/Resources/build-metadata.json" \
  "$APP_MARKETING_VERSION" \
  "$BUILD_COMMIT" \
  "$BUILD_TAG" \
  "$BUILD_STATE" \
  "$BUILD_SOURCE_URL"
cleanup_appledouble "$STAGING_APP_PATH"
cleanup_codesign_artifacts "$STAGING_APP_PATH"
normalize_bundle_permissions "$STAGING_APP_PATH"
rm -f "$STAGING_APP_PATH/Contents/embedded.provisionprofile"
xattr -cr "$STAGING_APP_PATH" 2>/dev/null || true

signing_arguments=(--force --deep --sign "$CODESIGN_IDENTITY" --options runtime)
if [[ "$CODESIGN_IDENTITY" == "-" ]]; then
  echo "以 ad-hoc 簽章簽署非沙盒 App Bundle..."
else
  echo "以 Developer ID Application 簽署非沙盒 App Bundle：$CODESIGN_IDENTITY"
  signing_arguments+=(--timestamp)
fi
codesign "${signing_arguments[@]}" "$STAGING_APP_PATH"
codesign --verify --deep --strict --verbose=2 "$STAGING_APP_PATH"

echo "複製 App 回專案目錄..."
ditto --norsrc --noextattr --noqtn "$STAGING_APP_PATH" "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

BUILT_BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_PATH/Contents/Info.plist")"
if [[ "$BUILT_BUNDLE_ID" != "$APP_BUNDLE_ID" ]]; then
  echo "建置失敗：Bundle ID 為 $BUILT_BUNDLE_ID，預期為 $APP_BUNDLE_ID"
  exit 1
fi

echo "完成：./dist/$APP_NAME.app"
echo "Bundle ID：$BUILT_BUNDLE_ID"
echo "來源版本：$BUILD_TAG ($BUILD_COMMIT, $BUILD_STATE)"
if [[ "$CODESIGN_IDENTITY" == "-" ]]; then
  echo "本機驗證版採 ad-hoc 簽章，不適合直接對外發布。"
else
  echo "Developer ID Application 簽章完成；仍須公證與 staple 後才能正式發布。"
fi
echo "公開版未啟用 App Sandbox，可直接存取目前帳號有權限的檔案與目錄。"

#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
APP_NAME="IntegTERM"
BUILD_BIN_DIR="$SCRIPT_DIR/build/bin"
APP_PATH="$BUILD_BIN_DIR/$APP_NAME.app"
TMP_ROOT=""
export MACOSX_DEPLOYMENT_TARGET="12.0"
export CGO_CFLAGS="-mmacosx-version-min=12.0"
export CGO_LDFLAGS="-mmacosx-version-min=12.0"
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1
CODESIGN_IDENTITY="${CODESIGN_IDENTITY:--}"
APP_BUNDLE_ID="${APP_BUNDLE_ID:-com.vader.integterm}"

if (( $# > 0 )); then
  echo "用法：$0"
  echo "公開版使用 ad-hoc 簽章，不啟用 App Sandbox。"
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

find_wails() {
  if command -v wails >/dev/null 2>&1; then
    command -v wails
    return 0
  fi

  local candidates=()
  local gopath=""
  gopath="$(go env GOPATH 2>/dev/null || true)"
  if [[ -n "$gopath" ]]; then
    candidates+=("$gopath/bin/wails")
  fi
  candidates+=(
    "$HOME/go/bin/wails"
    "/opt/homebrew/bin/wails"
    "/usr/local/bin/wails"
  )

  local candidate
  for candidate in "${candidates[@]}"; do
    if [[ -x "$candidate" ]]; then
      echo "$candidate"
      return 0
    fi
  done

  return 1
}

install_wails() {
  echo "未找到 Wails，正在自動安裝..."
  GO111MODULE=on go install github.com/wailsapp/wails/v2/cmd/wails@latest
}

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

WAILS_BIN="$(find_wails || true)"
if [[ -z "$WAILS_BIN" ]]; then
  install_wails
  WAILS_BIN="$(find_wails || true)"
fi

if [[ -z "$WAILS_BIN" ]]; then
  echo "缺少必要指令: wails"
  echo "可手動執行：go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  exit 1
fi

cd "$FRONTEND_DIR"
if [[ ! -d node_modules ]]; then
  echo "安裝前端依賴中..."
  npm install
fi

cd "$SCRIPT_DIR"
echo "整理 Go 模組..."
go mod tidy

echo "同步 App Icon..."
"$SCRIPT_DIR/sync-app-icon.sh"

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

cd "$SCRIPT_DIR"
mkdir -p "$BUILD_BIN_DIR"
rm -rf "$APP_PATH"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/integterm-build.XXXXXX")"
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
  "$SCRIPT_DIR/" "$STAGING_DIR/"

cleanup_appledouble "$STAGING_DIR"
cleanup_appledouble "$STAGING_DIR/frontend/dist"

if [[ ! -s "$STAGING_DIR/frontend/dist/index.html" ]]; then
  echo "建置失敗：暫存專案缺少 frontend/dist/index.html，無法嵌入 Wails 前端資產。"
  exit 1
fi

echo "開始打包 Wails 應用程式..."
(
  cd "$STAGING_DIR"
  "$WAILS_BIN" build -clean -s
)

if [[ ! -d "$STAGING_APP_PATH" ]]; then
  echo "建置失敗：找不到暫存產物 $STAGING_APP_PATH"
  exit 1
fi

/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $APP_BUNDLE_VERSION" "$STAGING_APP_PATH/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier $APP_BUNDLE_ID" "$STAGING_APP_PATH/Contents/Info.plist"
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

echo "完成：$APP_PATH"
echo "Bundle ID：$BUILT_BUNDLE_ID"
if [[ "$CODESIGN_IDENTITY" == "-" ]]; then
  echo "本機驗證版採 ad-hoc 簽章，不適合直接對外發布。"
else
  echo "Developer ID Application 簽章完成；仍須公證與 staple 後才能正式發布。"
fi
echo "公開版未啟用 App Sandbox，可直接存取目前帳號有權限的檔案與目錄。"

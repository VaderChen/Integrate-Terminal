#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
APP_NAME="IntegTERM"
BUILD_BIN_DIR="$SCRIPT_DIR/build/bin"
APP_PATH="$BUILD_BIN_DIR/$APP_NAME.app"
PKG_PATH="$BUILD_BIN_DIR/$APP_NAME.pkg"
TMP_ROOT=""
export MACOSX_DEPLOYMENT_TARGET="13.0"
export CGO_CFLAGS="-mmacosx-version-min=13.0"
export CGO_LDFLAGS="-mmacosx-version-min=13.0"
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1

cleanup_tmp_root() {
  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" ]]; then
    rm -rf "$TMP_ROOT"
  fi
}

trap cleanup_tmp_root EXIT

if [[ -f "$HOME/.zshrc" ]]; then
  source "$HOME/.zshrc"
fi

# 編譯版本統一取自 go.mod，不受使用者 shell 的全域工具鏈設定影響。
unset GOROOT
export GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' "$SCRIPT_DIR/go.mod")"
# Applies to Wails' bindings helper and dev builds as well as the final binary.
# Go also remaps CGO C/Objective-C source paths when trimpath is enabled.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath"

APP_MARKETING_VERSION="1.$(date +%y).$(date +%m%d)"
APP_BUILD_LABEL="$(date +%H%M)"
APP_DISPLAY_VERSION="$APP_MARKETING_VERSION build $APP_BUILD_LABEL"
APP_BUNDLE_VERSION="1.$(date +%y).$(date +%m%d%H%M)"
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

copy_storekit_bridge() {
  local app_path="$1"
  local bridge_path="$SCRIPT_DIR/internal/purchase/native/libintegtermstorekit2.dylib"
  local frameworks_dir="$app_path/Contents/Frameworks"
  if [[ ! -f "$bridge_path" ]]; then
    echo "找不到 StoreKit 2 橋接動態庫：$bridge_path"
    exit 1
  fi
  mkdir -p "$frameworks_dir"
  cp "$bridge_path" "$frameworks_dir/"
}

configure_storekit_bridge_rpath() {
  local app_path="$1"
  local executable_path="$app_path/Contents/MacOS/$APP_NAME"
  if [[ -f "$executable_path" ]]; then
    install_name_tool -add_rpath "@executable_path/../Frameworks" "$executable_path" 2>/dev/null || true
  fi
}

required_commands=(go node npm)
for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少必要指令: $cmd"
    echo "請先安裝完成後再執行 $0"
    exit 1
  fi
done

if ! command -v rsync >/dev/null 2>&1; then
  echo "缺少必要指令: rsync"
  echo "請先安裝完成後再執行 $0"
  exit 1
fi

WAILS_BIN="$(zsh "$SCRIPT_DIR/scripts/ensure-wails.sh")"

if ! command -v codesign >/dev/null 2>&1; then
  echo "缺少必要指令: codesign"
  echo "請先安裝完成後再執行 $0"
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

echo "建置 StoreKit 2 原生橋接..."
"$SCRIPT_DIR/scripts/build-storekit2-bridge.sh"
export DYLD_LIBRARY_PATH="$SCRIPT_DIR/internal/purchase/native${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"

echo "同步 App Icon..."
"$SCRIPT_DIR/sync-app-icon.sh"

echo "同步 Wails 產品版本..."
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
echo "清理前端舊產物..."
rm -rf dist
npm run build
cleanup_appledouble "$FRONTEND_DIR/dist"

cd "$SCRIPT_DIR"
mkdir -p "$BUILD_BIN_DIR"
echo "清理桌面應用舊產物..."
rm -rf "$APP_PATH" "$PKG_PATH"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/integterm-build.XXXXXX")"
STAGING_DIR="$TMP_ROOT/project"
STAGING_BUILD_BIN_DIR="$STAGING_DIR/build/bin"
STAGING_APP_PATH="$STAGING_BUILD_BIN_DIR/$APP_NAME.app"

echo "同步專案到本機暫存目錄..."
mkdir -p "$STAGING_DIR"
rsync -a \
  --exclude '.git/' \
  --exclude '.DS_Store' \
  --exclude '._*' \
  --exclude '*.bak' \
  --exclude 'build/bin/' \
  --exclude 'frontend/node_modules/.cache/' \
  "$SCRIPT_DIR/" "$STAGING_DIR/"

cleanup_appledouble "$STAGING_DIR"
cleanup_appledouble "$STAGING_DIR/frontend/dist"

echo "開始於本機暫存目錄打包 Wails 應用程式（production / App Store-safe build）..."
(
  cd "$STAGING_DIR"
  export CGO_CFLAGS="-mmacosx-version-min=13.0"
  export CGO_LDFLAGS="-mmacosx-version-min=13.0"
  export DYLD_LIBRARY_PATH="$STAGING_DIR/internal/purchase/native${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"
  "$WAILS_BIN" build -clean -s -trimpath
)

if [[ ! -d "$STAGING_APP_PATH" ]]; then
  echo "建置失敗：找不到暫存產物 $STAGING_APP_PATH"
  exit 1
fi

echo "更新 App Store 版本資訊..."
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $APP_BUNDLE_VERSION" "$STAGING_APP_PATH/Contents/Info.plist"

echo "清理暫存 bundle 中繼檔..."
cleanup_appledouble "$STAGING_APP_PATH"
cleanup_codesign_artifacts "$STAGING_APP_PATH"
normalize_bundle_permissions "$STAGING_APP_PATH"
copy_storekit_bridge "$STAGING_APP_PATH"
configure_storekit_bridge_rpath "$STAGING_APP_PATH"
xattr -cr "$STAGING_APP_PATH" 2>/dev/null || true

echo "重新簽署暫存 App Bundle..."
codesign --force --deep --sign - "$STAGING_APP_PATH"
codesign --verify --deep --strict --verbose=2 "$STAGING_APP_PATH"

echo "複製 App 回專案目錄..."
ditto "$STAGING_APP_PATH" "$APP_PATH"
cleanup_appledouble "$APP_PATH"
normalize_bundle_permissions "$APP_PATH"

echo "完成：$APP_PATH"

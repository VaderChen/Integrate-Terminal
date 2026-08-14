#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_NAME="IntegTERM"
STAMP="$(date +%Y%m%d)"
OUTPUT_NAME="${PROJECT_NAME}_${STAMP}.zip"
OUTPUT_PATH="$SCRIPT_DIR/$OUTPUT_NAME"
STAGING_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/integterm-backup.XXXXXX")"
STAGING_DIR="$STAGING_ROOT/$PROJECT_NAME"
APP_ICON_SOURCE="$SCRIPT_DIR/assets/appicon.png"

cleanup() {
  rm -rf "$STAGING_ROOT"
}
trap cleanup EXIT

required_commands=(rsync zip find)
for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少必要指令: $cmd"
    exit 1
  fi
done

if [[ ! -f "$APP_ICON_SOURCE" ]]; then
  echo "找不到 App Icon 原始資產：$APP_ICON_SOURCE"
  echo "請先確認 assets/appicon.png 存在後再執行備份。"
  exit 1
fi

mkdir -p "$STAGING_DIR"

# 排除建置產物、中繼檔、前端依賴與暫存包。
# 注意：保留 build/ 內的專案來源檔，例如 build/darwin/*.plist 與 build/appicon.png，
# 只排除真正的打包產物 build/bin/。
rsync -a \
  --exclude '.DS_Store' \
  --exclude '._*' \
  --exclude '.git/' \
  --exclude '.gitignore' \
  --exclude '.codex-tmp/' \
  --exclude 'build/bin/' \
  --exclude 'frontend/node_modules/' \
  --exclude 'frontend/dist/' \
  --exclude '*.zip' \
  --exclude '*.pkg' \
  --exclude '*.dmg' \
  --exclude '*.app' \
  --exclude '*.bak' \
  --exclude 'trayprobe' \
  --exclude 'IntegTERM' \
  "$SCRIPT_DIR/" "$STAGING_DIR/"

find "$STAGING_DIR" -name '.DS_Store' -delete
find "$STAGING_DIR" -name '._*' -delete

rm -f "$OUTPUT_PATH"
(
  cd "$STAGING_ROOT"
  zip -qry "$OUTPUT_PATH" "$PROJECT_NAME"
)

echo "備份完成：$OUTPUT_PATH"

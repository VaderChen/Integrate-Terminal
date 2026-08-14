#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_NAME="IntegTERM"
APP_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.app"
TARGET_DIR="${INSTALL_TARGET_DIR:-/Applications}"
TARGET_PATH="$TARGET_DIR/$APP_NAME.app"
APP_ICON_SOURCE="$SCRIPT_DIR/assets/appicon.png"
SKIP_BUILD=false

if [[ "${1:-}" == "--skip-build" ]]; then
  SKIP_BUILD=true
  shift
fi
if (( $# > 0 )); then
  echo "不支援的參數：$1"
  echo "用法：$0 [--skip-build]"
  exit 1
fi

required_commands=(codesign ditto)
for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "缺少必要指令: $cmd"
    echo "請先安裝完成後再執行 $0"
    exit 1
  fi
done

close_running_app() {
  local app_name="$1"
  local wait_attempt

  if pgrep -x "$app_name" >/dev/null 2>&1; then
    echo "偵測到 $app_name 正在執行，先嘗試正常關閉 ..."
    osascript -e "tell application \"$app_name\" to quit" >/dev/null 2>&1 || true

    for wait_attempt in {1..20}; do
      if ! pgrep -x "$app_name" >/dev/null 2>&1; then
        break
      fi
      sleep 0.5
    done
  fi

  if pgrep -x "$app_name" >/dev/null 2>&1; then
    echo "仍有 $app_name 行程未結束，改用強制關閉 ..."
    pkill -x "$app_name" >/dev/null 2>&1 || true

    for wait_attempt in {1..10}; do
      if ! pgrep -x "$app_name" >/dev/null 2>&1; then
        break
      fi
      sleep 0.3
    done
  fi

  if pgrep -x "$app_name" >/dev/null 2>&1; then
    echo "無法關閉執行中的 $app_name，安裝中止。"
    exit 1
  fi
}

if [[ ! -f "$APP_ICON_SOURCE" ]]; then
  echo "找不到 App Icon 原始資產：$APP_ICON_SOURCE"
  echo "請先確認 assets/appicon.png 存在後再執行 $0"
  exit 1
fi

if [[ "$SKIP_BUILD" == "false" ]]; then
  echo "重新建置並套用舊 App 固定簽章..."
  "$SCRIPT_DIR/build.sh"
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "找不到已打包的 App：$APP_PATH"
  exit 1
fi

signature_details="$(codesign -dvv "$APP_PATH" 2>&1 || true)"
designated_requirement="$(codesign -d -r- "$APP_PATH" 2>&1 || true)"
if ! grep -Fq "TeamIdentifier=8QB2QM35YM" <<< "$signature_details" || \
   ! grep -Fq 'identifier "com.vader.integterm" and anchor apple generic' <<< "$designated_requirement"; then
  echo "安裝中止：$APP_PATH 未使用舊 App Team 身份簽章。"
  echo "請移除 --skip-build 後重新執行。"
  exit 1
fi

if [[ ! -d "$TARGET_DIR" ]]; then
  echo "建立安裝目錄：$TARGET_DIR"
  mkdir -p "$TARGET_DIR"
fi

close_running_app "$APP_NAME"

echo "安裝到 $TARGET_DIR ..."
rm -rf "$TARGET_PATH"
ditto "$APP_PATH" "$TARGET_PATH"

codesign --verify --deep --strict --verbose=2 "$TARGET_PATH"

echo "完成：$TARGET_PATH"

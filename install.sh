#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_NAME="IntegTERM"
APP_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.app"
TARGET_DIR="${INSTALL_TARGET_DIR:-/Applications}"
TARGET_PATH="$TARGET_DIR/$APP_NAME.app"
APP_ICON_SOURCE="$SCRIPT_DIR/assets/appicon.png"

required_commands=(ditto)
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

if [[ ! -d "$APP_PATH" || "$APP_ICON_SOURCE" -nt "$APP_PATH" ]]; then
  if [[ -d "$APP_PATH" ]]; then
    echo "偵測到 App Icon 已更新，先重新執行 ./build.sh ..."
  else
    echo "找不到已打包的 App：$APP_PATH"
    echo "先執行 ./build.sh ..."
  fi
  "$SCRIPT_DIR/build.sh"
fi

if [[ ! -d "$TARGET_DIR" ]]; then
  echo "建立安裝目錄：$TARGET_DIR"
  mkdir -p "$TARGET_DIR"
fi

close_running_app "$APP_NAME"

echo "安裝到 $TARGET_DIR ..."
rm -rf "$TARGET_PATH"
ditto "$APP_PATH" "$TARGET_PATH"

echo "完成：$TARGET_PATH"

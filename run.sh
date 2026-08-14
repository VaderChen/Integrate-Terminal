#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
TMP_ROOT=""
SYNC_WATCHER_PID=""
export MACOSX_DEPLOYMENT_TARGET="12.0"
export CGO_CFLAGS="-mmacosx-version-min=12.0"
export CGO_LDFLAGS="-mmacosx-version-min=12.0"
export VITE_APP_VERSION="1.$(date +%y).$(date +%m%d) build $(date +%H%M)"
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1

cleanup_dev_runtime() {
  if [[ -n "$SYNC_WATCHER_PID" ]]; then
    pkill -P "$SYNC_WATCHER_PID" >/dev/null 2>&1 || true
    kill "$SYNC_WATCHER_PID" >/dev/null 2>&1 || true
    wait "$SYNC_WATCHER_PID" 2>/dev/null || true
  fi
  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" ]]; then
    local temp_process_pid
    for temp_process_pid in $(pgrep -f "$TMP_ROOT" 2>/dev/null || true); do
      [[ "$temp_process_pid" == "$$" ]] || kill "$temp_process_pid" >/dev/null 2>&1 || true
    done
    local cleanup_attempt
    for cleanup_attempt in {1..10}; do
      rm -rf "$TMP_ROOT" 2>/dev/null || true
      [[ ! -e "$TMP_ROOT" ]] && break
      sleep 0.1
    done
  fi
}

trap cleanup_dev_runtime EXIT INT TERM

if [[ -f "$HOME/.zshrc" ]]; then
  source "$HOME/.zshrc"
fi

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
  local install_target="github.com/wailsapp/wails/v2/cmd/wails@latest"
  GO111MODULE=on go install "$install_target"
}

required_commands=(go node npm rsync)
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
  echo "已嘗試自動安裝，但仍未找到。"
  echo "可手動執行：go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  echo "若已安裝，請確認 \$HOME/go/bin 或 \$(go env GOPATH)/bin 已加入 PATH。"
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

TMP_BASE="${TMPDIR:-/tmp}"
TMP_ROOT="$(mktemp -d "${TMP_BASE%/}/integterm-dev.XXXXXX")"
STAGING_DIR="$TMP_ROOT/project"

sync_project_to_local() {
  rsync -rlc --delete \
    --exclude '.git/' \
    --exclude '.DS_Store' \
    --exclude '._*' \
    --exclude 'build/bin/' \
    --exclude 'frontend/dist/' \
    --exclude 'frontend/node_modules/' \
    --exclude 'frontend/wailsjs/' \
    "$SCRIPT_DIR/" "$STAGING_DIR/"
}

echo "建立本機開發鏡像：$STAGING_DIR"
mkdir -p "$STAGING_DIR/frontend"
sync_project_to_local

if [[ -d "$FRONTEND_DIR/node_modules" ]]; then
  rm -rf "$STAGING_DIR/frontend/node_modules"
  ln -s "$FRONTEND_DIR/node_modules" "$STAGING_DIR/frontend/node_modules"
fi
rm -rf "$STAGING_DIR/frontend/wailsjs"
ln -s "$FRONTEND_DIR/wailsjs" "$STAGING_DIR/frontend/wailsjs"

if [[ "${INTEGTERM_DEV_PREPARE_ONLY:-0}" == "1" ]]; then
  echo "本機開發鏡像準備完成。"
  exit 0
fi

echo "監看原專案並同步到本機開發鏡像..."
(
  while true; do
    sync_project_to_local
    sleep 0.4
  done
) &
SYNC_WATCHER_PID=$!

echo "啟動 Wails 開發模式（本機暫存目錄）..."
cd "$STAGING_DIR"
"$WAILS_BIN" dev -m -nosyncgomod

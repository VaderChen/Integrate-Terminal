#!/bin/zsh

set -euo pipefail

cd "$(dirname "$0")"

FRONTEND_DIR="./frontend"
TMP_ROOT=""
SYNC_WATCHER_PID=""
export MACOSX_DEPLOYMENT_TARGET="12.0"
export CGO_CFLAGS="-mmacosx-version-min=12.0"
export CGO_LDFLAGS="-mmacosx-version-min=12.0"
export VITE_APP_VERSION="1.$(date +%y).$(date +%m%d) build $(date +%H%M)"
export COPYFILE_DISABLE=1
export COPY_EXTENDED_ATTRIBUTES_DISABLE=1

MULTI_INSTANCE=0
for argument in "$@"; do
  case "$argument" in
    --multi-instance)
      MULTI_INSTANCE=1
      ;;
    *)
      echo "用法：$0 [--multi-instance]"
      exit 1
      ;;
  esac
done

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

required_commands=(go node npm rsync)
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

mkdir -p "./.codex-tmp"
TMP_ROOT="$(mktemp -d "./.codex-tmp/integterm-dev.XXXXXX")"
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
    ./ "$STAGING_DIR/"
}

echo "建立本機開發鏡像：$STAGING_DIR"
mkdir -p "$STAGING_DIR/frontend"
sync_project_to_local

if [[ -d "$FRONTEND_DIR/node_modules" ]]; then
  rm -rf "$STAGING_DIR/frontend/node_modules"
  ln -s "../../../../frontend/node_modules" "$STAGING_DIR/frontend/node_modules"
fi
rm -rf "$STAGING_DIR/frontend/wailsjs"
ln -s "../../../../frontend/wailsjs" "$STAGING_DIR/frontend/wailsjs"

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
WAILS_DEV_ARGS=(dev -m -nosyncgomod -skipembedcreate)
if (( MULTI_INSTANCE == 1 )); then
  WAILS_DEV_ARGS+=(-appargs "--multi-instance")
fi
(
  cd "$STAGING_DIR"
  "${WAILS_COMMAND[@]}" "${WAILS_DEV_ARGS[@]}"
)

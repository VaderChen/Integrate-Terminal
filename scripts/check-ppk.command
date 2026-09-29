#!/bin/zsh

# 密語只經由標準輸入傳遞，不寫入參數、環境變數、歷史紀錄或檔案。
set -eu
set +x
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CHECK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/integterm-ppk-check.XXXXXX")"
trap 'unset PPK_CHECK_PASSPHRASE; rm -rf "$CHECK_DIR"' EXIT
PPK_CHECK_BIN="$SCRIPT_DIR/ppk-check"
if [[ ! -x "$PPK_CHECK_BIN" ]]; then
  cd "$PROJECT_DIR"
  unset GOROOT
  export GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' go.mod)"
  echo '建立 PPK 本機診斷工具…'
  go build -o "$CHECK_DIR/ppk-check" ./cmd/ppk-check
  PPK_CHECK_BIN="$CHECK_DIR/ppk-check"
fi

PPK_CHECK_FILE="${1:-}"
if [[ -z "$PPK_CHECK_FILE" ]]; then
  IFS= read -r 'PPK_CHECK_FILE?請貼上 PPK 完整路徑（不加引號）：'
fi
IFS= read -rs 'PPK_CHECK_PASSPHRASE?PPK 密語（未加密請直接按 Enter，輸入不顯示）：'
echo
CHECK_RESULT=0
printf '%s' "$PPK_CHECK_PASSPHRASE" | "$PPK_CHECK_BIN" -file "$PPK_CHECK_FILE" || CHECK_RESULT=$?
unset PPK_CHECK_PASSPHRASE
read -r '?按 Enter 結束。'
exit "$CHECK_RESULT"

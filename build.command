#!/bin/zsh

set -u

cd "$(dirname "$0")"

if ./build.sh "$@"; then
  APP_PATH="./dist/IntegTERM.app"
  if [[ -d "$APP_PATH" ]] && command -v open >/dev/null 2>&1; then
    open -R "$APP_PATH" >/dev/null 2>&1 || true
  fi
else
  exit_code=$?
  echo "建置失敗，錯誤代碼：$exit_code；請檢查上方訊息。"
  exit "$exit_code"
fi

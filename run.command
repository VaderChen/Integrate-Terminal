#!/bin/zsh

set -u

cd "$(dirname "$0")"

if [[ "${1:-}" == "--dev" ]]; then
  shift
  exec ./run.sh "$@"
fi

APP_PATH="./dist/IntegTERM.app"
if [[ ! -d "$APP_PATH" ]]; then
  echo "找不到建置產物：$APP_PATH"
  echo "正在先執行建置..."
  ./build.sh
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "建置完成後仍找不到 App：$APP_PATH"
  exit 1
fi

if (( $# > 0 )); then
  exec open "$APP_PATH" --args "$@"
fi
exec open "$APP_PATH"

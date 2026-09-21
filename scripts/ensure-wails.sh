#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"
unset GOROOT
export GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' go.mod)"

# CLI 的套件分析器必須能讀取專案工具鏈產生的型別資訊。
# 與專案選用相同的 Wails、Go 和 x/tools，避免 PATH 中的舊 CLI 被誤用。
WAILS_MODULE="github.com/wailsapp/wails/v2"
WAILS_VERSION="$(go list -m -f '{{.Version}}' "$WAILS_MODULE")"
TOOLS_VERSION="$(go list -m -f '{{.Version}}' golang.org/x/tools)"
GO_VERSION="$(go env GOVERSION)"
GO_PLATFORM="$(go env GOOS)-$(go env GOARCH)"
CACHE_ROOT="${INTEGTERM_TOOL_CACHE:-${XDG_CACHE_HOME:-$HOME/Library/Caches}/IntegTERM/tools}"
CACHE_DIR="$CACHE_ROOT/wails-$WAILS_VERSION-$GO_VERSION-$TOOLS_VERSION-$GO_PLATFORM"
mkdir -p "$CACHE_DIR"
CACHE_DIR="$(cd "$CACHE_DIR" && pwd)"
WAILS_BIN="$CACHE_DIR/wails"

matches_project() {
  local binary="$1"
  local metadata=""
  [[ -x "$binary" ]] || return 1
  metadata="$(go version -m "$binary" 2>/dev/null)" || return 1
  [[ "$(awk 'NR == 1 { print $NF }' <<< "$metadata")" == "$GO_VERSION" ]] || return 1
  [[ "$(awk -v module="$WAILS_MODULE" '$1 == "mod" && $2 == module { print $3 }' <<< "$metadata")" == "$WAILS_VERSION" ]] || return 1
  [[ "$(awk '$1 == "dep" && $2 == "golang.org/x/tools" { print $3 }' <<< "$metadata")" == "$TOOLS_VERSION" ]]
}

if matches_project "$WAILS_BIN"; then
  print -r -- "$WAILS_BIN"
  exit 0
fi

BUILD_DIR="$(mktemp -d "$CACHE_DIR/.build.XXXXXX")"
trap 'rm -rf "$BUILD_DIR"' EXIT

print -r -- "建立專案 Wails CLI：$WAILS_VERSION / $GO_VERSION / x/tools $TOOLS_VERSION" >&2
cat > "$BUILD_DIR/go.mod" <<EOF
module integterm.tools/wails

go ${GO_VERSION#go}

require (
  $WAILS_MODULE $WAILS_VERSION
  golang.org/x/tools $TOOLS_VERSION
)
EOF

# 工具依賴放在獨立模組，避免修改應用程式的 go.mod、go.sum 或全域 CLI。
(
  cd "$BUILD_DIR"
  GOWORK=off go build -mod=mod -trimpath -o "$BUILD_DIR/wails" "$WAILS_MODULE/cmd/wails"
) >&2

if ! matches_project "$BUILD_DIR/wails"; then
  print -r -- "Wails CLI 的編譯版本與專案不符，停止啟動。" >&2
  exit 1
fi
mv -f "$BUILD_DIR/wails" "$WAILS_BIN"
print -r -- "$WAILS_BIN"

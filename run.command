#!/bin/zsh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
exec "$SCRIPT_DIR/run.sh" "$@"

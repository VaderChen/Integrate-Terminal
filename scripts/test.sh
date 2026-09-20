#!/bin/zsh

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
unset GOROOT
export GOTOOLCHAIN="$(awk '$1 == "go" { print "go" $2; exit }' "$PROJECT_DIR/go.mod")"
TEST_RUNTIME="$(mktemp -d /tmp/integterm-test.XXXXXX)"
trap 'rm -rf "$TEST_RUNTIME"' EXIT

cd "$PROJECT_DIR"
if [[ ! -f internal/purchase/native/libintegtermstorekit2.dylib ]]; then
  "$SCRIPT_DIR/build-storekit2-bridge.sh"
fi
cp internal/purchase/native/libintegtermstorekit2.dylib "$TEST_RUNTIME/"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -Wl,-rpath,$TEST_RUNTIME"
go test -race -count=1 -timeout=120s ./...
go vet ./...
python3 "$SCRIPT_DIR/test-packaging.py"
python3 "$SCRIPT_DIR/tests/test_sandbox_probe.py"
python3 "$PROJECT_DIR/internal/purchase/swift/test_bridge.py"
cd frontend
npm test
npm exec tsc -- --noEmit

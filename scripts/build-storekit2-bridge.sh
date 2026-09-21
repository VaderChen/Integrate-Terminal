#!/usr/bin/env zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SOURCE_FILE="$PROJECT_DIR/internal/purchase/swift/StoreKit2Bridge.swift"
OUTPUT_DIR="$PROJECT_DIR/internal/purchase/native"
OUTPUT_LIB="$OUTPUT_DIR/libintegtermstorekit2.dylib"

if [[ ! -f "$SOURCE_FILE" ]]; then
  echo "找不到 Swift 橋接原始碼：$SOURCE_FILE"
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

SDK_PATH="$(xcrun --sdk macosx --show-sdk-path)"
TARGET_ARCH="$(uname -m)"
TARGET_TRIPLE="${TARGET_ARCH}-apple-macos13.0"

# Relative input and stable source mappings keep the builder's home/volume out
# of Swift runtime locations, debug metadata and imported Clang source paths.
cd "$PROJECT_DIR"
xcrun --sdk macosx swiftc \
  -parse-as-library \
  -emit-library \
  -module-name IntegTERMStoreKit2Bridge \
  -gnone \
  -file-compilation-dir . \
  -file-prefix-map "$PROJECT_DIR=/IntegTERM" \
  -Xcc "-ffile-prefix-map=$PROJECT_DIR=/IntegTERM" \
  -target "$TARGET_TRIPLE" \
  -sdk "$SDK_PATH" \
  -o "$OUTPUT_LIB" \
  "internal/purchase/swift/StoreKit2Bridge.swift"

install_name_tool -id "@rpath/libintegtermstorekit2.dylib" "$OUTPUT_LIB"

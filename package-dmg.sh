#!/bin/zsh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_NAME="IntegTERM"
APP_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.app"
DIST_DIR="${DIST_DIR:-$SCRIPT_DIR/dist}"
STAGING_DIR=""
PUBLISH_DIR=""

cleanup_staging() {
  if [[ -n "$STAGING_DIR" && -d "$STAGING_DIR" ]]; then
    rm -rf "$STAGING_DIR"
  fi
  if [[ -n "$PUBLISH_DIR" && -d "$PUBLISH_DIR" ]]; then
    rm -rf "$PUBLISH_DIR"
  fi
}
trap cleanup_staging EXIT

if ! command -v hdiutil >/dev/null 2>&1; then
  echo "缺少必要指令: hdiutil"
  echo "請先安裝完成後再執行 $0"
  exit 1
fi

if [[ ! -x /usr/libexec/PlistBuddy ]]; then
  echo "找不到必要工具: /usr/libexec/PlistBuddy"
  echo "請先安裝完成後再執行 $0"
  exit 1
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "找不到已打包的 App：$APP_PATH"
  echo "先執行 ./build.sh ..."
  "$SCRIPT_DIR/build.sh"
fi

VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP_PATH/Contents/Info.plist")"
DMG_PATH="$DIST_DIR/$APP_NAME-$VERSION.dmg"
VOLUME_NAME="$APP_NAME $VERSION"

mkdir -p "$DIST_DIR"
# Build the image on the local filesystem. External filesystems can lack the
# metadata and symlink support hdiutil needs while scanning the source folder.
STAGING_DIR="$(mktemp -d /tmp/integterm-dmg.XXXXXX)"
DMG_ROOT="$STAGING_DIR/$APP_NAME-root"
mkdir -p "$DMG_ROOT"

ditto --norsrc --noextattr --noqtn "$APP_PATH" "$DMG_ROOT/$APP_NAME.app"
ln -s /Applications "$DMG_ROOT/Applications"
STAGED_DMG="$STAGING_DIR/$APP_NAME.dmg"

echo "建立 DMG：$DMG_PATH"
hdiutil create \
  -volname "$VOLUME_NAME" \
  -srcfolder "$DMG_ROOT" \
  -ov \
  -format UDZO \
  "$STAGED_DMG"

# Finish the cross-volume copy before replacing the existing release. The final
# rename stays on the destination filesystem, so a failed copy preserves it.
PUBLISH_DIR="$(mktemp -d "$DIST_DIR/.dmg-publish.XXXXXX")"
PUBLISHED_DMG="$PUBLISH_DIR/$APP_NAME.dmg"
ditto --norsrc --noextattr --noqtn "$STAGED_DMG" "$PUBLISHED_DMG"
mv -f "$PUBLISHED_DMG" "$DMG_PATH"

echo "完成：$DMG_PATH"

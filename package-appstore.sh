#!/usr/bin/env zsh

# Build a production app before packaging; debug/devtools builds are unsuitable
# for App Store submission. Local signing settings: scripts/README-signing.md.
set -euo pipefail
umask 077

APP_NAME="IntegTERM"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_HELPER="$SCRIPT_DIR/scripts/appstore_config.py"
SIGNING_CONFIG="${SIGNING_CONFIG:-$SCRIPT_DIR/cert/signing.json}"
command -v python3 >/dev/null || { echo "需要 python3 讀取簽章設定。" >&2; exit 1; }
read_setting() { python3 "$CONFIG_HELPER" value "$SIGNING_CONFIG" "$1"; }
TEAM_ID="${TEAM_ID:-$(read_setting TEAM_ID)}"
APP_CERT="${APP_CERT:-$(read_setting APP_CERT)}"
INSTALLER_CERT="${INSTALLER_CERT:-$(read_setting INSTALLER_CERT)}"
BUNDLE_ID="${BUNDLE_ID:-$(read_setting BUNDLE_ID)}"
BUNDLE_ID="${BUNDLE_ID:-com.vader.integterm}"
PROVISION_PROFILE_PATH="${PROVISION_PROFILE_PATH:-$(read_setting PROVISION_PROFILE_PATH)}"
CERT_DIR="${CERT_DIR:-$(read_setting CERT_DIR)}"
CERT_DIR="${CERT_DIR:-$SCRIPT_DIR/cert}"
SIGNING_KEYCHAIN="${SIGNING_KEYCHAIN:-$(read_setting SIGNING_KEYCHAIN)}"
SIGNING_KEYCHAIN="${SIGNING_KEYCHAIN:-$HOME/Library/Keychains/login.keychain-db}"
APP_SOURCE_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.app"
STAGING_DIR=""
APP_PATH=""
ENTITLEMENTS_TEMPLATE="$SCRIPT_DIR/entitlements.plist"
PKG_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.pkg"
PROFILE_SEARCH_DIR="$HOME/Library/MobileDevice/Provisioning Profiles"
TEMP_KEYCHAIN=""
TEMP_KEYCHAIN_DIR=""
TEMP_KEYCHAIN_PASSWORD="${TEMP_KEYCHAIN_PASSWORD:-}"
export MACOSX_DEPLOYMENT_TARGET="12.0"

required_commands=(codesign productbuild pkgutil security xattr ditto mktemp uuidgen)
for cmd in "${required_commands[@]}"; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "缺少必要指令: $cmd" >&2; exit 1; }
done

cleanup() {
    [[ ${ZSH_SUBSHELL:-0} -eq 0 ]] || return 0
    if [[ -n "$TEMP_KEYCHAIN" && -f "$TEMP_KEYCHAIN" ]]; then
        security delete-keychain "$TEMP_KEYCHAIN" >/dev/null 2>&1 || true
    fi
    [[ -z "$TEMP_KEYCHAIN_DIR" || ! -d "$TEMP_KEYCHAIN_DIR" ]] || rm -rf "$TEMP_KEYCHAIN_DIR"
    [[ -z "$STAGING_DIR" || ! -d "$STAGING_DIR" ]] || rm -rf "$STAGING_DIR"
    return 0
}
# zsh ERR_EXIT within a function may bypass the outer EXIT trap.
trap cleanup EXIT ZERR

if [[ ! -d "$APP_SOURCE_PATH" ]]; then
    "$SCRIPT_DIR/build.sh"
fi
[[ -d "$APP_SOURCE_PATH" && -f "$ENTITLEMENTS_TEMPLATE" ]] || { echo "找不到正式 App 或 entitlements 範本。" >&2; exit 1; }
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/integterm-appstore.XXXXXX")"
PROFILE_PLIST="$STAGING_DIR/profile.plist"
ENTITLEMENTS="$STAGING_DIR/entitlements.plist"

if [[ -z "$PROVISION_PROFILE_PATH" && -f "$CERT_DIR/IntegTerm.provisionprofile" ]]; then
    PROVISION_PROFILE_PATH="$CERT_DIR/IntegTerm.provisionprofile"
fi
if [[ -z "$PROVISION_PROFILE_PATH" ]]; then
    for profile in "$PROFILE_SEARCH_DIR"/*.provisionprofile(.N) "$PROFILE_SEARCH_DIR"/*.mobileprovision(.N); do
        if security cms -D -i "$profile" > "$PROFILE_PLIST" 2>/dev/null && \
            python3 "$CONFIG_HELPER" matches --profile "$PROFILE_PLIST" --bundle-id "$BUNDLE_ID" --team-id "$TEAM_ID" 2>/dev/null; then
            PROVISION_PROFILE_PATH="$profile"
            break
        fi
    done
fi
[[ -n "$PROVISION_PROFILE_PATH" && -f "$PROVISION_PROFILE_PATH" ]] || { echo "請用 PROVISION_PROFILE_PATH 指定 macOS distribution provisioning profile。" >&2; exit 1; }
security cms -D -i "$PROVISION_PROFILE_PATH" > "$PROFILE_PLIST"
TEAM_ID="$(python3 "$CONFIG_HELPER" prepare --profile "$PROFILE_PLIST" --template "$ENTITLEMENTS_TEMPLATE" --output "$ENTITLEMENTS" --app-info "$APP_SOURCE_PATH/Contents/Info.plist" --bundle-id "$BUNDLE_ID" --team-id "$TEAM_ID")"

import_local_signing_assets() {
    local -a assets=()
    local asset
    if [[ -f "$CERT_DIR/distribution.key" && -f "$CERT_DIR/distribution.cer" ]]; then
        assets+=("$CERT_DIR/distribution.key" "$CERT_DIR/distribution.cer")
    elif [[ -f "$CERT_DIR/distribution.p12" ]]; then
        assets+=("$CERT_DIR/distribution.p12")
    else
        for asset in distribution.key distribution.cer; do
            [[ ! -f "$CERT_DIR/$asset" ]] || assets+=("$CERT_DIR/$asset")
        done
    fi
    if [[ -f "$CERT_DIR/installer.p12" ]]; then
        assets+=("$CERT_DIR/installer.p12")
    elif [[ -f "$CERT_DIR/installer.cer" ]]; then
        assets+=("$CERT_DIR/installer.cer")
    fi
    (( ${#assets[@]} > 0 )) || return 0
    TEMP_KEYCHAIN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/integterm-signing.XXXXXX")"
    TEMP_KEYCHAIN="$TEMP_KEYCHAIN_DIR/signing.keychain-db"
    TEMP_KEYCHAIN_PASSWORD="${TEMP_KEYCHAIN_PASSWORD:-$(uuidgen)}"
    security create-keychain -p "$TEMP_KEYCHAIN_PASSWORD" "$TEMP_KEYCHAIN" >/dev/null
    SIGNING_KEYCHAIN="$TEMP_KEYCHAIN"
    security unlock-keychain -p "$TEMP_KEYCHAIN_PASSWORD" "$TEMP_KEYCHAIN" >/dev/null
    security set-keychain-settings -lut 21600 "$TEMP_KEYCHAIN" >/dev/null
    for asset in "${assets[@]}"; do
        if [[ "$asset" == *.p12 ]]; then
            security import "$asset" -k "$TEMP_KEYCHAIN" -P "${CERT_P12_PASSWORD:-}" \
                -T /usr/bin/codesign -T /usr/bin/productbuild -T /usr/bin/pkgbuild >/dev/null
        else
            security import "$asset" -k "$TEMP_KEYCHAIN" \
                -T /usr/bin/codesign -T /usr/bin/productbuild -T /usr/bin/pkgbuild >/dev/null
        fi
    done
    security set-key-partition-list -S apple-tool:,apple:,codesign:,productbuild:,pkgbuild: \
        -s -k "$TEMP_KEYCHAIN_PASSWORD" "$TEMP_KEYCHAIN" >/dev/null
}

import_local_signing_assets
APP_PATH="$STAGING_DIR/$APP_NAME.app"
ditto "$APP_SOURCE_PATH" "$APP_PATH"
APP_CERT="$(security find-identity -v -p codesigning "$SIGNING_KEYCHAIN" | python3 "$CONFIG_HELPER" identity --kind app --team-id "$TEAM_ID" --preferred "$APP_CERT" --profile "$PROFILE_PLIST")"
INSTALLER_CERT="$(security find-identity -v "$SIGNING_KEYCHAIN" | python3 "$CONFIG_HELPER" identity --kind installer --team-id "$TEAM_ID" --preferred "$INSTALLER_CERT" --profile "$PROFILE_PLIST")"

find "$APP_PATH" -name '._*' -delete
find "$APP_PATH" -name '*.cstemp' -delete
find "$APP_PATH" -name '_CodeSignature' -type d -prune -exec rm -rf {} +
find "$APP_PATH" -type d -exec chmod 755 {} +
find "$APP_PATH" -type f -exec chmod 644 {} +
chmod 755 "$APP_PATH/Contents/MacOS/$APP_NAME"
xattr -cr "$APP_PATH"
cp "$PROVISION_PROFILE_PATH" "$APP_PATH/Contents/embedded.provisionprofile"
chmod 644 "$APP_PATH/Contents/embedded.provisionprofile"
find "$APP_PATH" -name '._*' -delete
xattr -cr "$APP_PATH"

codesign --force --deep --verbose --keychain "$SIGNING_KEYCHAIN" --sign "$APP_CERT" \
    --entitlements "$ENTITLEMENTS" --options runtime "$APP_PATH"
productbuild --component "$APP_PATH" /Applications --keychain "$SIGNING_KEYCHAIN" \
    --sign "$INSTALLER_CERT" "$PKG_PATH"
pkgutil --check-signature "$PKG_PATH"
codesign --verify --deep --strict "$APP_PATH"
if xattr -lr "$APP_PATH" 2>/dev/null | grep -q "com.apple.quarantine"; then
    echo "仍偵測到 quarantine 屬性，請檢查來源 App。" >&2
    exit 1
fi
echo "完成：build/bin/$APP_NAME.pkg，可使用 Transporter 上傳。"

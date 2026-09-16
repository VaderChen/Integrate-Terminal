#!/usr/bin/env zsh

# 警告：執行此腳本前，請確保已在 Apple Developer Portal 建立證書
# 1. Apple Distribution: "Apple Distribution: YOUR_CERTIFICATE_NAME (YOUR_TEAM_ID)"
# 2. Mac Installer Distribution: "Mac Installer Distribution: YOUR_CERTIFICATE_NAME (YOUR_TEAM_ID)"
# 注意：提交 App Store 時不可使用 `wails build -debug` 或 `-devtools`，
# 否則會包含 WebKit inspector 的 private API 並被警告/拒絕。
#
# 如果您需要透過 CLI 產生憑證請求 (CSR) 以下載證書，請執行：
# openssl genrsa -out distribution.key 2048
# openssl req -new -key distribution.key -out distribution.csr -subj "/CN=YOUR_CERTIFICATE_NAME/OU=YOUR_TEAM_ID/C=TW"
# 然後將 distribution.csr 上傳至 Apple Developer Portal

set -euo pipefail

# --- 設定變數 ---
APP_NAME="IntegTERM"
TEAM_ID="YOUR_TEAM_ID" # 使用您提供的 Team ID
# 注意：App Store 證書名稱可能是 "Apple Distribution" 或 "3rd Party Mac Developer Application"
APP_CERT="3rd Party Mac Developer Application: YOUR_CERTIFICATE_NAME ($TEAM_ID)" 
INSTALLER_CERT="3rd Party Mac Developer Installer: YOUR_CERTIFICATE_NAME ($TEAM_ID)" 
BUNDLE_ID="com.vader.integterm" # 使用您提供的 Bundle ID

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_SOURCE_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.app"
CERT_DIR="$SCRIPT_DIR/cert"
STAGING_DIR=""
APP_PATH=""
ENTITLEMENTS="$SCRIPT_DIR/entitlements.plist"
PKG_PATH="$SCRIPT_DIR/build/bin/$APP_NAME.pkg"
PROFILE_SEARCH_DIR="$HOME/Library/MobileDevice/Provisioning Profiles"
DEFAULT_PROFILE_PATH="$SCRIPT_DIR/cert/IntegTerm.provisionprofile"
LOGIN_KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
TEMP_KEYCHAIN="$HOME/Library/Keychains/integterm-appstore-signing.keychain-db"
TEMP_KEYCHAIN_PASSWORD="${TEMP_KEYCHAIN_PASSWORD:-integterm-appstore}"
PROVISION_PROFILE_PATH="${PROVISION_PROFILE_PATH:-}"
export MACOSX_DEPLOYMENT_TARGET="12.0"

# --- 檢查環境 ---
if [[ "$TEAM_ID" == "YOUR_TEAM_ID" ]]; then
    echo "請先編輯此腳本，設定您的 TEAM_ID, APP_CERT, INSTALLER_CERT 和 BUNDLE_ID"
    exit 1
fi

required_commands=(codesign productbuild pkgutil security xattr)
for cmd in "${required_commands[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "缺少必要指令: $cmd"
        echo "請先安裝完成後再執行 $0"
        exit 1
    fi
done

if [[ ! -d "$APP_SOURCE_PATH" ]]; then
    echo "找不到 App：$APP_SOURCE_PATH，正在執行 build.sh..."
    ./build.sh
fi

if [[ ! -d "$APP_SOURCE_PATH" ]]; then
    echo "找不到 App：$APP_SOURCE_PATH"
    exit 1
fi

find_matching_profile() {
    local profile
    for profile in "$PROFILE_SEARCH_DIR"/*.provisionprofile(.N) "$PROFILE_SEARCH_DIR"/*.mobileprovision(.N); do
        if security cms -D -i "$profile" 2>/dev/null | grep -q "<string>$BUNDLE_ID</string>"; then
            echo "$profile"
            return 0
        fi
    done
    return 1
}

if [[ -z "$PROVISION_PROFILE_PATH" ]]; then
    if [[ -f "$DEFAULT_PROFILE_PATH" ]]; then
        PROVISION_PROFILE_PATH="$DEFAULT_PROFILE_PATH"
    fi
fi

if [[ -z "$PROVISION_PROFILE_PATH" ]]; then
    if [[ -d "$PROFILE_SEARCH_DIR" ]]; then
        PROVISION_PROFILE_PATH="$(find_matching_profile || true)"
    fi
fi

if [[ -z "$PROVISION_PROFILE_PATH" ]]; then
    echo "找不到對應 $BUNDLE_ID 的 provisioning profile。"
    echo "請先安裝 Mac App Store / TestFlight 用的 provisioning profile，或用以下方式指定："
    echo "PROVISION_PROFILE_PATH=/path/to/profile.provisionprofile ./package-appstore.sh"
    exit 1
fi

if [[ ! -f "$PROVISION_PROFILE_PATH" ]]; then
    echo "指定的 provisioning profile 不存在：$PROVISION_PROFILE_PATH"
    exit 1
fi

if [[ ! -f "$ENTITLEMENTS" ]]; then
    echo "找不到 entitlements 檔案：$ENTITLEMENTS"
    exit 1
fi

identity_exists_codesigning() {
    local preferred="$1"
    if security find-identity -v -p codesigning | grep -Fq "\"$preferred\""; then
        return 0
    fi
    return 1
}

identity_exists_any() {
    local preferred="$1"
    if security find-identity -v | grep -Fq "\"$preferred\""; then
        return 0
    fi
    return 1
}

restore_keychain_search_list() {
    if [[ -n "${ORIGINAL_KEYCHAINS_RAW:-}" ]]; then
        local -a keychains=()
        while IFS= read -r line; do
            line="${line#\"}"
            line="${line%\"}"
            [[ -n "$line" ]] && keychains+=("$line")
        done <<< "$ORIGINAL_KEYCHAINS_RAW"

        if (( ${#keychains[@]} > 0 )); then
            security list-keychains -d user -s "${keychains[@]}" >/dev/null
        fi
    fi
}

cleanup_temp_keychain() {
    if [[ ${ZSH_SUBSHELL:-0} -gt 0 ]]; then
        return 0
    fi
    if [[ -n "${ORIGINAL_DEFAULT_KEYCHAIN:-}" ]]; then
        security default-keychain -d user -s "$ORIGINAL_DEFAULT_KEYCHAIN" >/dev/null 2>&1 || true
    fi
    restore_keychain_search_list
    if [[ -f "$TEMP_KEYCHAIN" ]]; then
        security delete-keychain "$TEMP_KEYCHAIN" >/dev/null 2>&1 || true
    fi
    if [[ -n "$STAGING_DIR" && -d "$STAGING_DIR" ]]; then
        rm -rf "$STAGING_DIR"
    fi
}

import_local_signing_assets() {
    if [[ ! -d "$CERT_DIR" ]]; then
        return 0
    fi

    echo "檢查並匯入 ./cert 內的簽章資產..."
    find "$CERT_DIR" -name '._*' -delete 2>/dev/null || true

    ORIGINAL_KEYCHAINS_RAW="$(security list-keychains -d user | tr -d ' ')"
    ORIGINAL_DEFAULT_KEYCHAIN="$(security default-keychain -d user 2>/dev/null | tr -d '"' || true)"
    rm -f "$TEMP_KEYCHAIN"
    security create-keychain -p "$TEMP_KEYCHAIN_PASSWORD" "$TEMP_KEYCHAIN" >/dev/null
    security unlock-keychain -p "$TEMP_KEYCHAIN_PASSWORD" "$TEMP_KEYCHAIN" >/dev/null
    security set-keychain-settings -lut 21600 "$TEMP_KEYCHAIN" >/dev/null

    if [[ ! -f "$CERT_DIR/distribution.key" || ! -f "$CERT_DIR/distribution.cer" ]]; then
        if [[ -n "${CERT_P12_PASSWORD:-}" && -f "$CERT_DIR/distribution.p12" ]]; then
            security import "$CERT_DIR/distribution.p12" \
                -k "$TEMP_KEYCHAIN" \
                -P "$CERT_P12_PASSWORD" \
                -T /usr/bin/codesign \
                -T /usr/bin/productbuild \
                -T /usr/bin/pkgbuild >/dev/null 2>&1 || true
        fi
    fi

    if [[ ! -f "$CERT_DIR/installer.cer" ]]; then
        if [[ -n "${CERT_P12_PASSWORD:-}" && -f "$CERT_DIR/installer.p12" ]]; then
            security import "$CERT_DIR/installer.p12" \
                -k "$TEMP_KEYCHAIN" \
                -P "$CERT_P12_PASSWORD" \
                -T /usr/bin/codesign \
                -T /usr/bin/productbuild \
                -T /usr/bin/pkgbuild >/dev/null 2>&1 || true
        fi
    fi

    if [[ -f "$CERT_DIR/distribution.key" ]]; then
        security import "$CERT_DIR/distribution.key" \
            -k "$TEMP_KEYCHAIN" \
            -A \
            -T /usr/bin/codesign \
            -T /usr/bin/productbuild \
            -T /usr/bin/pkgbuild >/dev/null 2>&1 || true
    fi

    if [[ -f "$CERT_DIR/distribution.cer" ]]; then
        security import "$CERT_DIR/distribution.cer" \
            -k "$TEMP_KEYCHAIN" \
            -T /usr/bin/codesign \
            -T /usr/bin/productbuild \
            -T /usr/bin/pkgbuild >/dev/null 2>&1 || true
    fi

    if [[ -f "$CERT_DIR/installer.cer" ]]; then
        security import "$CERT_DIR/installer.cer" \
            -k "$TEMP_KEYCHAIN" \
            -T /usr/bin/codesign \
            -T /usr/bin/productbuild \
            -T /usr/bin/pkgbuild >/dev/null 2>&1 || true
    fi

    security set-key-partition-list \
        -S apple-tool:,apple:,codesign:,productbuild:,pkgbuild: \
        -s \
        -k "$TEMP_KEYCHAIN_PASSWORD" \
        "$TEMP_KEYCHAIN" >/dev/null 2>&1 || true

    security default-keychain -d user -s "$TEMP_KEYCHAIN" >/dev/null
    security list-keychains -d user -s "$TEMP_KEYCHAIN" "$LOGIN_KEYCHAIN" >/dev/null
    security find-identity -v "$TEMP_KEYCHAIN" >/dev/null 2>&1 || true
}

find_valid_identity() {
    local preferred="$1"
    if identity_exists_codesigning "$preferred"; then
        echo "$preferred"
        return 0
    fi
    return 1
}

find_installer_identity() {
    local preferred="$1"
    if identity_exists_any "$preferred"; then
        echo "$preferred"
        return 0
    fi

    local candidate
    for candidate in \
        "Mac Installer Distribution: YOUR_CERTIFICATE_NAME ($TEAM_ID)" \
        "3rd Party Mac Developer Installer: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    do
        if identity_exists_any "$candidate"; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

find_app_identity() {
    local preferred="$1"
    if security find-identity -v -p codesigning | grep -Fq "\"$preferred\""; then
        echo "$preferred"
        return 0
    fi

    local candidate
    for candidate in \
        "Apple Distribution: YOUR_CERTIFICATE_NAME ($TEAM_ID)" \
        "3rd Party Mac Developer Application: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    do
        if security find-identity -v -p codesigning | grep -Fq "\"$candidate\""; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

cleanup_appledouble() {
    local target="$1"
    find "$target" -name '._*' -print -delete
}

cleanup_codesign_artifacts() {
    local target="$1"
    find "$target" -name '*.cstemp' -print -delete
    find "$target" -name '_CodeSignature' -type d -prune -print -exec rm -rf {} +
}

normalize_bundle_permissions() {
    local target="$1"
    find "$target" -type d -exec chmod 755 {} +
    find "$target" -type f -exec chmod 644 {} +
    if [[ -f "$target/Contents/MacOS/$APP_NAME" ]]; then
        chmod 755 "$target/Contents/MacOS/$APP_NAME"
    fi
}

prepare_staging_app() {
    STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/integterm-appstore.XXXXXX")"
    APP_PATH="$STAGING_DIR/$APP_NAME.app"
    ditto "$APP_SOURCE_PATH" "$APP_PATH"
}

import_local_signing_assets
prepare_staging_app
EMBEDDED_PROFILE_PATH="$APP_PATH/Contents/embedded.provisionprofile"

APP_CERT="$(find_app_identity "$APP_CERT" || true)"
INSTALLER_CERT="$(find_installer_identity "$INSTALLER_CERT" || true)"
trap cleanup_temp_keychain EXIT

if [[ -z "$APP_CERT" ]]; then
    echo "找不到可用的 App Store Application 簽章身份。"
    echo "請先安裝以下其中一種證書到 Keychain："
    echo "  - Apple Distribution: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    echo "  - 3rd Party Mac Developer Application: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    exit 1
fi

if [[ -z "$INSTALLER_CERT" ]]; then
    echo "找不到可用的 App Store Installer 簽章身份。"
    echo "請先安裝以下其中一種證書到 Keychain："
    echo "  - Mac Installer Distribution: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    echo "  - 3rd Party Mac Developer Installer: YOUR_CERTIFICATE_NAME ($TEAM_ID)"
    exit 1
fi

echo "使用 App 簽章身份：$APP_CERT"
echo "使用 Installer 簽章身份：$INSTALLER_CERT"

echo "清理 AppleDouble 汙染檔..."
cleanup_appledouble "$APP_PATH"
echo "清理舊簽章殘留..."
cleanup_codesign_artifacts "$APP_PATH"
echo "修正 App Bundle 權限..."
normalize_bundle_permissions "$APP_PATH"

# 1. 移除不允許的延伸屬性 (Resource Forks, etc.)
# 這些屬性會導致簽名失敗
echo "清理延伸屬性..."
cleanup_appledouble "$APP_PATH"
xattr -cr "$APP_PATH" 2>/dev/null || true

echo "嵌入 provisioning profile..."
cp "$PROVISION_PROFILE_PATH" "$EMBEDDED_PROFILE_PATH"
chmod 644 "$EMBEDDED_PROFILE_PATH"
echo "使用 provisioning profile：$PROVISION_PROFILE_PATH"
echo "再次清理 AppleDouble 汙染檔..."
cleanup_appledouble "$APP_PATH"
echo "清理嵌入後的延伸屬性..."
xattr -cr "$APP_PATH" 2>/dev/null || true

# 2. 簽名 App Bundle
# Wails 產生的 binary 在 Contents/MacOS/$APP_NAME
# 必須包含 --entitlements 並使用 --options runtime (雖然 App Store 主要是 Sandboxing)
codesign --force --deep --verbose \
    --keychain "$TEMP_KEYCHAIN" \
    --sign "$APP_CERT" \
    --entitlements "$ENTITLEMENTS" \
    --options runtime \
    "$APP_PATH"

echo "--- 建立安裝套件 (.pkg) ---"

# 3. 使用 productbuild 建立專供 App Store 提交的 pkg
productbuild --component "$APP_PATH" /Applications \
    --keychain "$TEMP_KEYCHAIN" \
    --sign "$INSTALLER_CERT" \
    "$PKG_PATH"

echo "--- 驗證簽名 ---"
pkgutil --check-signature "$PKG_PATH"
echo "--- 驗證 Provisioning Profile ---"
ls -l "$EMBEDDED_PROFILE_PATH"
echo "--- 驗證無 quarantine 屬性 ---"
cleanup_appledouble "$APP_PATH"
if xattr -lr "$APP_PATH" 2>/dev/null | grep -q "com.apple.quarantine"; then
    echo "仍偵測到 com.apple.quarantine，請檢查來源檔案。"
    exit 1
fi
echo "--- 驗證 Entitlements ---"
codesign -d --entitlements :- "$APP_PATH" 2>/dev/null | plutil -p -

echo "完成！您可以將 $PKG_PATH 上傳至 App Store Connect。"
echo "建議使用 'Transporter' 出現或經由 Xcode -> Project -> Archive (若有 Xcode 專案) 上傳。"

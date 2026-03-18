#!/bin/bash
set -e

rm -rf bin
rm -rf dist

# Full build: Go binaries + frontend + Electron app
#
# Usage: ./build.sh              (current platform)
#        ./build.sh darwin arm64  (specific platform)
#        ./build.sh all           (all platforms)
#        ./build.sh --dev         (current platform, skip signing/notarization)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Parse --dev flag
DEV_MODE=false
ARGS=()
for arg in "$@"; do
    if [ "$arg" = "--dev" ]; then
        DEV_MODE=true
    else
        ARGS+=("$arg")
    fi
done
set -- "${ARGS[@]}"

BINARIES=(grroxy grroxy-app grroxy-tool cook)

ALL_PLATFORMS=(
    "darwin:arm64"
    "darwin:amd64"
    "linux:amd64"
    "linux:arm64"
    "windows:amd64"
)

# --- Signing config (macOS only) ---
# Requires:
#   - Certificate "Developer ID Application" installed in Keychain
#   - Notarization credentials stored via:
#     xcrun notarytool store-credentials "grroxy-notarize" ...

CODESIGN_IDENTITY="Developer ID Application: Gitesh Sharma (96Q778FZG7)"
NOTARIZE_PROFILE="grroxy-notarize"

SIGN_ENABLED=false
if [ "$DEV_MODE" = true ]; then
    echo "Dev mode: skipping signing/notarization"
elif security find-identity -v -p codesigning 2>/dev/null | grep -q "$CODESIGN_IDENTITY"; then
    SIGN_ENABLED=true
    echo "Code signing enabled: ${CODESIGN_IDENTITY}"
else
    echo "Warning: Signing certificate not found, skipping signing/notarization"
fi

codesign_binary() {
    local BINARY_PATH="$1"
    if [ "$SIGN_ENABLED" = true ] && file "$BINARY_PATH" | grep -q "Mach-O"; then
        codesign --force --options runtime --timestamp \
            --sign "$CODESIGN_IDENTITY" "$BINARY_PATH"
        echo "    Signed: $(basename "$BINARY_PATH")"
    fi
}

notarize_dmg() {
    local DMG_PATH="$1"
    if [ "$SIGN_ENABLED" != true ]; then
        echo "  Skipping notarization (signing not configured)"
        return
    fi

    echo "  Submitting for notarization: $(basename "$DMG_PATH")"
    xcrun notarytool submit "$DMG_PATH" \
        --keychain-profile "$NOTARIZE_PROFILE" \
        --wait

    echo "  Stapling: $(basename "$DMG_PATH")"
    xcrun stapler staple "$DMG_PATH"
    echo "  Done: $(basename "$DMG_PATH")"
}

notarize_zip() {
    local ZIP_PATH="$1"
    if [ "$SIGN_ENABLED" != true ]; then return; fi

    echo "  Submitting for notarization: $(basename "$ZIP_PATH")"
    xcrun notarytool submit "$ZIP_PATH" \
        --keychain-profile "$NOTARIZE_PROFILE" \
        --wait
    echo "  Done: $(basename "$ZIP_PATH")"
}

build_go_platform() {
    local TARGET_OS="$1"
    local TARGET_ARCH="$2"

    local EXT=""
    if [ "$TARGET_OS" = "windows" ]; then
        EXT=".exe"
    fi

    # electron-builder ${os} resolves to: mac, win, linux
    local PLATFORM_DIR="$TARGET_OS"
    if [ "$TARGET_OS" = "darwin" ]; then
        PLATFORM_DIR="mac"
    elif [ "$TARGET_OS" = "windows" ]; then
        PLATFORM_DIR="win"
    fi

    # Go uses amd64, Node/Electron uses x64
    local ARCH_DIR="$TARGET_ARCH"
    if [ "$TARGET_ARCH" = "amd64" ]; then
        ARCH_DIR="x64"
    fi

    local OUT_DIR="${SCRIPT_DIR}/bin/${PLATFORM_DIR}/${ARCH_DIR}"
    mkdir -p "$OUT_DIR"

    echo "Building Go binaries for ${TARGET_OS}/${TARGET_ARCH} -> ${OUT_DIR}"

    for binary in "${BINARIES[@]}"; do
        printf "  %s ..." "$binary"
        local PKG="${PROJECT_ROOT}/cmd/${binary}"
        if [ "$binary" = "cook" ]; then
            PKG="github.com/glitchedgitz/cook/v2/cmd/cook"
        fi
        GOOS=$TARGET_OS GOARCH=$TARGET_ARCH CGO_ENABLED=0 go build \
            -o "${OUT_DIR}/${binary}${EXT}" \
            "$PKG"
        echo " OK"

        # Sign macOS binaries immediately after build
        codesign_binary "${OUT_DIR}/${binary}${EXT}"
    done

    echo
}

# --- Step 1: Build Go binaries ---

echo "=== Step 1: Go binaries ==="

if [ "${1:-}" = "all" ]; then
    for platform in "${ALL_PLATFORMS[@]}"; do
        IFS=: read -r os arch <<< "$platform"
        build_go_platform "$os" "$arch"
    done
else
    TARGET_OS="${1:-$(go env GOOS)}"
    TARGET_ARCH="${2:-$(go env GOARCH)}"
    build_go_platform "$TARGET_OS" "$TARGET_ARCH"
fi

# --- Step 2: Install npm deps if needed ---

echo "=== Step 2: npm install ==="
cd "$SCRIPT_DIR"
npm install

# --- Step 3: Package Electron app ---

echo "=== Step 3: Package Electron app ==="

# electron-builder picks up these env vars for signing
if [ "$SIGN_ENABLED" = true ]; then
    export CSC_NAME="Gitesh Sharma (96Q778FZG7)"
else
    export CSC_IDENTITY_AUTO_DISCOVERY=false
fi
# Skip Windows signing
export WIN_CSC_LINK=""

electron_arch_flag() {
    case "$1" in
        arm64) echo "--arm64" ;;
        amd64) echo "--x64" ;;
        *)     echo "--$1" ;;
    esac
}

electron_os_flag() {
    case "$1" in
        darwin)  echo "--mac" ;;
        linux)   echo "--linux" ;;
        windows) echo "--win" ;;
        *)       echo "--$1" ;;
    esac
}

if [ "${1:-}" = "all" ]; then
    npx electron-builder --mac --x64 --arm64
    npx electron-builder --linux --x64 --arm64
    npx electron-builder --win --x64
else
    OS_FLAG=$(electron_os_flag "$TARGET_OS")
    ARCH_FLAG=$(electron_arch_flag "$TARGET_ARCH")
    npx electron-builder $OS_FLAG $ARCH_FLAG
fi

# --- Step 4: Rename mac artifacts to include chip name ---

echo "=== Step 4: Rename mac artifacts ==="
for f in "${SCRIPT_DIR}"/dist/grroxy-mac-arm64-*; do
    [ -e "$f" ] || continue
    newname="${f/mac-arm64-/mac-arm64-applechip-}"
    echo "  Renaming $(basename "$f") -> $(basename "$newname")"
    mv "$f" "$newname"
done
for f in "${SCRIPT_DIR}"/dist/grroxy-mac-x64-*; do
    [ -e "$f" ] || continue
    newname="${f/mac-x64-/mac-x64-intelchip-}"
    echo "  Renaming $(basename "$f") -> $(basename "$newname")"
    mv "$f" "$newname"
done

# --- Step 5: Notarize macOS artifacts ---

echo "=== Step 5: Notarize macOS artifacts ==="
NOTARIZE_PIDS=()
for f in "${SCRIPT_DIR}"/dist/*.dmg; do
    [ -e "$f" ] || continue
    notarize_dmg "$f" &
    NOTARIZE_PIDS+=($!)
done
for f in "${SCRIPT_DIR}"/dist/*-mac-*.zip; do
    [ -e "$f" ] || continue
    notarize_zip "$f" &
    NOTARIZE_PIDS+=($!)
done

# Wait for all notarizations to finish
for pid in "${NOTARIZE_PIDS[@]}"; do
    wait "$pid" || { echo "Notarization failed (PID $pid)"; exit 1; }
done

echo
echo "=== Done ==="
echo "Output in: ${SCRIPT_DIR}/dist/"
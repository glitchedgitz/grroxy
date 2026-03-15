#!/bin/bash
set -e

rm -r bin
rm -r dist

# Full build: Go binaries + frontend + Electron app
#
# Usage: ./build.sh              (current platform)
#        ./build.sh darwin arm64  (specific platform)
#        ./build.sh all           (all platforms)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BINARIES=(grroxy grroxy-app grroxy-tool cook)

ALL_PLATFORMS=(
    "darwin:arm64"
    "darwin:amd64"
    "linux:amd64"
    "linux:arm64"
    "windows:amd64"
)

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
if [ "${1:-}" = "all" ]; then
    npx electron-builder --mac --x64 --arm64
    npx electron-builder --linux --x64 --arm64
    npx electron-builder --win --x64
else
    npm run build
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

echo
echo "=== Done ==="
echo "Output in: ${SCRIPT_DIR}/dist/"

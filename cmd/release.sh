#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/.."

VERSION=$(cat "$PROJECT_ROOT/VERSION")
DIST="$PROJECT_ROOT/dist"
CMDS=("grroxy" "grroxy-app" "grroxy-tool")
PLATFORMS=("darwin/arm64" "darwin/amd64" "linux/amd64" "linux/arm64" "windows/amd64")

# The AI CLI bridges are staged from the frontend repo. They carry per-platform
# native binaries (Codex) so each archive gets its own install; set
# GRROXY_SKIP_BRIDGES=1 to cut the archives down to the Go binaries alone.
BRIDGE_BUILDER="${GRROXY_BRIDGE_BUILDER:-$PROJECT_ROOT/../cybernetic-ui/build-bridges.sh}"
SKIP_BRIDGES="${GRROXY_SKIP_BRIDGES:-0}"

if [ "$SKIP_BRIDGES" != "1" ] && [ ! -x "$BRIDGE_BUILDER" ]; then
  echo "warning: $BRIDGE_BUILDER not found or not executable; archives will omit the AI bridges." >&2
  echo "         Set GRROXY_BRIDGE_BUILDER, or GRROXY_SKIP_BRIDGES=1 to silence this." >&2
  SKIP_BRIDGES=1
fi

# Go's GOOS/GOARCH vocabulary differs from npm's --os/--cpu.
npm_os() {
  case "$1" in
    windows) echo "win32" ;;
    *) echo "$1" ;;
  esac
}
npm_cpu() {
  case "$1" in
    amd64) echo "x64" ;;
    *) echo "$1" ;;
  esac
}

rm -rf "$DIST"
mkdir -p "$DIST"

for platform in "${PLATFORMS[@]}"; do
  os="${platform%/*}"
  arch="${platform#*/}"
  dir="grroxy-${VERSION}-${os}-${arch}"
  outdir="${DIST}/${dir}"
  mkdir -p "$outdir"

  ext=""
  if [ "$os" = "windows" ]; then
    ext=".exe"
  fi

  echo "Building ${os}/${arch}..."
  for cmd in "${CMDS[@]}"; do
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "${outdir}/${cmd}${ext}" "$PROJECT_ROOT/cmd/${cmd}"
  done

  # grroxy resolves bridges at <exeDir>/bridges/<name>/server.mjs
  if [ "$SKIP_BRIDGES" != "1" ]; then
    echo "  Staging AI bridges for $(npm_os "$os")/$(npm_cpu "$arch")..."
    "$BRIDGE_BUILDER" \
      --out "${outdir}/bridges" \
      --platform "$(npm_os "$os")" \
      --arch "$(npm_cpu "$arch")"
  fi

  # Package
  archive=""
  pushd "$DIST" > /dev/null
  if [ "$os" = "windows" ]; then
    archive="${dir}.zip"
    zip -rq "$archive" "$dir"
  else
    archive="${dir}.tar.gz"
    tar -czf "$archive" "$dir"
  fi
  popd > /dev/null

  rm -rf "$outdir"
  echo "  -> ${DIST}/${archive}"
done

echo ""
echo "Release archives:"
ls -lh "$DIST"/*.{tar.gz,zip} 2>/dev/null

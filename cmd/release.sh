#!/usr/bin/env bash
set -euo pipefail

VERSION=$(cat VERSION)
DIST="dist"
CMDS=("grroxy" "grroxy-app" "grroxy-tool")
PLATFORMS=("darwin/arm64" "darwin/amd64" "linux/amd64" "linux/arm64" "windows/amd64")

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
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "${outdir}/${cmd}${ext}" "./cmd/${cmd}"
  done

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

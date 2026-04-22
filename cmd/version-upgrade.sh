#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/.."

usage() {
  echo "Usage:"
  echo "  $0                                                       Bump patch (X.Y.Z -> X.Y.Z+1) for app, backend, frontend"
  echo "  $0 --major                                               Bump minor (X.Y.Z -> X.Y+1.0) for app, backend, frontend"
  echo "  $0 --undo                                                Discard version file changes (git checkout) in both repos"
  echo "  $0 <app_version> <backend_version> <frontend_version>    Set explicit versions"
  echo ""
  echo "Examples:"
  echo "  $0"
  echo "  $0 --major"
  echo "  $0 --undo"
  echo "  $0 2026.4.3 0.29.1 0.29.2"
  exit 1
}

SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+$'

VERSION_FILE="$PROJECT_ROOT/VERSION"
VERSION_GO="$PROJECT_ROOT/grx/version/version.go"
ELECTRON_PKG="$PROJECT_ROOT/cmd/electron/package.json"
CYB_ROOT="$PROJECT_ROOT/../cybernetic-ui"
CYB_VERSION_FILE="$CYB_ROOT/VERSION"
CYB_PKG="$CYB_ROOT/package.json"
CYB_APP_TS="$CYB_ROOT/src/lib/pages/app.ts"

bump_minor() {
  local v="$1"
  local major="${v%%.*}"
  local rest="${v#*.}"
  local minor="${rest%%.*}"
  echo "${major}.$((minor + 1)).0"
}

bump_patch() {
  local v="$1"
  local major="${v%%.*}"
  local rest="${v#*.}"
  local minor="${rest%%.*}"
  local patch="${rest#*.}"
  echo "${major}.${minor}.$((patch + 1))"
}

read_current() {
  CUR_APP=$(sed -n 's/^const RELEASED_APP_VERSION = "\(.*\)"$/\1/p' "$VERSION_GO")
  CUR_BACKEND=$(sed -n 's/^const RELEASED_BACKEND_VERSION = "\(.*\)"$/\1/p' "$VERSION_GO")
  CUR_FRONTEND=$(sed -n 's/^const RELEASED_FRONTEND_VERSION = "\(.*\)"$/\1/p' "$VERSION_GO")

  [[ "$CUR_APP" =~ $SEMVER_RE ]] || { echo "Could not parse current app version: '$CUR_APP'"; exit 1; }
  [[ "$CUR_BACKEND" =~ $SEMVER_RE ]] || { echo "Could not parse current backend version: '$CUR_BACKEND'"; exit 1; }
  [[ "$CUR_FRONTEND" =~ $SEMVER_RE ]] || { echo "Could not parse current frontend version: '$CUR_FRONTEND'"; exit 1; }
}

if [ "$#" -eq 1 ] && [ "$1" = "--undo" ]; then
  echo "Undoing version changes..."
  git -C "$PROJECT_ROOT" checkout -- VERSION grx/version/version.go cmd/electron/package.json
  echo "  reverted grroxy: VERSION, grx/version/version.go, cmd/electron/package.json"
  if [ -d "$CYB_ROOT/.git" ] || git -C "$CYB_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    git -C "$CYB_ROOT" checkout -- VERSION package.json src/lib/pages/app.ts
    echo "  reverted cybernetic-ui: VERSION, package.json, src/lib/pages/app.ts"
  else
    echo "  warning: $CYB_ROOT not a git repo, skipping"
  fi
  echo "Done."
  exit 0
fi

if [ "$#" -eq 0 ]; then
  read_current
  APP=$(bump_patch "$CUR_APP")
  BACKEND=$(bump_patch "$CUR_BACKEND")
  FRONTEND=$(bump_patch "$CUR_FRONTEND")
elif [ "$#" -eq 1 ] && [ "$1" = "--major" ]; then
  read_current
  APP=$(bump_minor "$CUR_APP")
  BACKEND=$(bump_minor "$CUR_BACKEND")
  FRONTEND=$(bump_minor "$CUR_FRONTEND")
elif [ "$#" -eq 3 ]; then
  APP="$1"
  BACKEND="$2"
  FRONTEND="$3"
  [[ "$APP" =~ $SEMVER_RE ]] || { echo "Invalid app version: $APP (want X.Y.Z)"; exit 1; }
  [[ "$BACKEND" =~ $SEMVER_RE ]] || { echo "Invalid backend version: $BACKEND (want X.Y.Z)"; exit 1; }
  [[ "$FRONTEND" =~ $SEMVER_RE ]] || { echo "Invalid frontend version: $FRONTEND (want X.Y.Z)"; exit 1; }
else
  usage
fi

sed_inplace() {
  sed -i.bak "$1" "$2"
  rm -f "${2}.bak"
}

echo "Bumping grroxy versions:"
echo "  app:      $APP"
echo "  backend:  $BACKEND"
echo "  frontend: $FRONTEND"

echo "v${APP}" > "$VERSION_FILE"
echo "  updated VERSION"

sed_inplace "s/^const CURRENT_BACKEND_VERSION = \".*\"/const CURRENT_BACKEND_VERSION = \"${BACKEND}\"/" "$VERSION_GO"
sed_inplace "s/^const CURRENT_FRONTEND_VERSION = \".*\"/const CURRENT_FRONTEND_VERSION = \"${FRONTEND}\"/" "$VERSION_GO"
sed_inplace "s/^const RELEASED_APP_VERSION = \".*\"/const RELEASED_APP_VERSION = \"${APP}\"/" "$VERSION_GO"
sed_inplace "s/^const RELEASED_BACKEND_VERSION = \".*\"/const RELEASED_BACKEND_VERSION = \"${BACKEND}\"/" "$VERSION_GO"
sed_inplace "s/^const RELEASED_FRONTEND_VERSION = \".*\"/const RELEASED_FRONTEND_VERSION = \"${FRONTEND}\"/" "$VERSION_GO"
echo "  updated grx/version/version.go"

sed_inplace "s/\"version\": \"[^\"]*\"/\"version\": \"${APP}\"/" "$ELECTRON_PKG"
echo "  updated cmd/electron/package.json"

if [ -d "$CYB_ROOT" ]; then
  echo "v${FRONTEND}" > "$CYB_VERSION_FILE"
  echo "  updated ../cybernetic-ui/VERSION"
  sed_inplace "s/\"version\": \"[^\"]*\"/\"version\": \"${FRONTEND}\"/" "$CYB_PKG"
  echo "  updated ../cybernetic-ui/package.json"
  if [ -f "$CYB_APP_TS" ]; then
    sed_inplace "s/export const VERSION = 'v[^']*'/export const VERSION = 'v${APP}'/" "$CYB_APP_TS"
    echo "  updated ../cybernetic-ui/src/lib/pages/app.ts"
  fi
else
  echo "  warning: $CYB_ROOT not found, skipping cybernetic-ui"
fi

echo "Done."

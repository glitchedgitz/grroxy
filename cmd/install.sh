#!/bin/bash

echo "Installing grroxy components..."

# Get the directory where the script is located (works in both Windows and Unix)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Store the original directory
ORIGINAL_DIR=$(pwd)

# Define the project root directory (one level up from cmd/)
PROJECT_ROOT="$SCRIPT_DIR/.."

# Array of directories to process - relative to cmd/
DIRS=("grroxy" "grroxy-app" "grroxy-tool" "grxp" "grx-fuzzer")

# Loop through each directory
for dir in "${DIRS[@]}"; do
    FULL_PATH="$PROJECT_ROOT/cmd/$dir"
    echo "Installing in cmd/$dir..."
    if [ ! -d "$FULL_PATH" ]; then
        echo "Directory $dir not found at $FULL_PATH"
        continue
    fi
    cd "$FULL_PATH" || { echo "Failed to enter $FULL_PATH"; continue; }
    go install || echo "Failed to install in $dir"
    cd "$ORIGINAL_DIR" || exit
done

# # --- AI CLI bridges ---
# #
# # `go install` puts binaries in $GOPATH/bin, and grroxy looks for bridges at
# # <exeDir>/bridges/<name>/server.mjs. Its dev fallback (walking up from the cwd)
# # can't help here because grroxy-app chdirs into the project directory at
# # startup, far away from this checkout. So link them into place explicitly.
# #
# # Symlinks, not copies: the bridges are ~425MB installed, and linking means edits
# # in the frontend repo take effect on the next bridge restart.

# BRIDGE_SRC="${GRROXY_BRIDGE_SRC:-$PROJECT_ROOT/../cybernetic-ui}"
# BIN_DIR="$(go env GOPATH)/bin"
# BRIDGES=("claude-code-bridge" "codex-bridge")

# if [ -d "$BRIDGE_SRC" ]; then
#     echo "Linking AI bridges from $BRIDGE_SRC..."
#     mkdir -p "$BIN_DIR/bridges"

#     for bridge in "${BRIDGES[@]}"; do
#         SRC="$(cd "$BRIDGE_SRC/$bridge" 2>/dev/null && pwd)" || {
#             echo "  $bridge: not found, skipping"
#             continue
#         }

#         if [ ! -d "$SRC/node_modules" ]; then
#             echo "  $bridge: installing dependencies (one-time, this is large)..."
#             (cd "$SRC" && npm install --no-audit --no-fund) || {
#                 echo "  $bridge: npm install failed"
#                 continue
#             }
#         fi

#         ln -sfn "$SRC" "$BIN_DIR/bridges/$bridge"
#         echo "  $bridge -> $BIN_DIR/bridges/$bridge"
#     done
# else
#     echo "Skipping AI bridges: $BRIDGE_SRC not found."
#     echo "  Set GRROXY_BRIDGE_SRC to your cybernetic-ui checkout to enable them."
# fi

# cd "$ORIGINAL_DIR" || exit

# echo "Installation complete!"
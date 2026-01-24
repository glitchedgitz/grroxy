#!/bin/bash

echo "Installing grroxy components..."

# Get the directory where the script is located (works in both Windows and Unix)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Store the original directory
ORIGINAL_DIR=$(pwd)

# Define the project root directory
PROJECT_ROOT="$SCRIPT_DIR"

# Array of directories to process - using Unix-style paths
DIRS=("cmd/grroxy" "cmd/grroxy-app" "cmd/grroxy-tool" "cmd/grxp" "cmd/grx-fuzzer")

# Loop through each directory
for dir in "${DIRS[@]}"; do
    FULL_PATH="$PROJECT_ROOT/$dir"
    echo "Installing in $dir..."
    if [ ! -d "$FULL_PATH" ]; then
        echo "Directory $dir not found at $FULL_PATH"
        continue
    fi
    cd "$FULL_PATH" || { echo "Failed to enter $FULL_PATH"; continue; }
    go install || echo "Failed to install in $dir"
    cd "$ORIGINAL_DIR" || exit
done

echo "Installation complete!" 
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

echo "Installation complete!" 
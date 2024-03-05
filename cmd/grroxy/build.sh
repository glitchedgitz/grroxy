#!/bin/bash

# Take version as input
VERSION=$1

# Check if version is provided
if [ -z $VERSION ]
then
    echo "Version is required"
    exit 1
fi

# Define the list of target platforms (OS/ARCH)
TARGETS=(
    "linux/arm"
    "linux/arm64"
    "linux/amd64"
    "linux/386"
    "darwin/amd64"
    "darwin/arm64"
    "windows/386"
    "windows/arm"
    "windows/amd64"
    "windows/arm64"
)

# Name of the Go package (replace with your package name)
# Name of the binary (replace with your binary name)
BINARY_NAME="grroxy"

# Function to build binaries for each target
build_binary() {
    echo "Building for $1"
    GOOS=${1%/*}
    GOARCH=${1#*/}
    GO111MODULE=on GOOS=$GOOS GOARCH=$GOARCH go build -o builds/$VERSION/$BINARY_NAME-$GOOS-$GOARCH
}

# Loop through each target and build the binary
for TARGET in "${TARGETS[@]}"
do
    build_binary $TARGET
done

echo "All binaries built successfully"
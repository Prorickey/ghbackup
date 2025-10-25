#!/usr/bin/env bash

set -e

BINARY_NAME="ghbackup"
OUTPUT_DIR="dist"

mkdir -p "$OUTPUT_DIR"

declare -a OS_LIST=("linux" "darwin" "windows")
declare -a ARCH_LIST=("amd64" "arm64")

echo "Building $BINARY_NAME for multiple platforms..."

for OS in "${OS_LIST[@]}"; do
  for ARCH in "${ARCH_LIST[@]}"; do
    OUT_NAME="${BINARY_NAME}-${OS}-${ARCH}"
    [[ $OS == "windows" ]] && OUT_NAME="${OUT_NAME}.exe"

    echo "Building $OUT_NAME ..."
    
    GOOS=$OS GOARCH=$ARCH go build -o "$OUTPUT_DIR/$OUT_NAME"

  done
done

echo "All binaries built successfully in $OUTPUT_DIR"
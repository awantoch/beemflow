#!/bin/bash

set -e

echo "Building BeemFlow Visual Editor WASM module..."

# Set WASM build environment
export GOOS=js
export GOARCH=wasm

# Build the WASM module
go build -o ../public/main.wasm main.go

# Copy Go's WASM support JavaScript
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ../public/

echo "WASM module built successfully!"
echo "Files created:"
echo "  - ../public/main.wasm"
echo "  - ../public/wasm_exec.js"
#!/bin/bash
# Build and install the Quotron CLI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building Quotron CLI..."
go build -o quotron ./cmd/main

echo "Installing to ../quotron binary..."
cp quotron ..

echo "Build completed successfully!"
echo "You can run the CLI with: ./quotron [command] [options]"
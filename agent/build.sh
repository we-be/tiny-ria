#!/bin/bash

set -e

# Ensure we're in the agent directory
cd "$(dirname "$0")"

echo "Building RIA unified tool..."
mkdir -p bin

# Create directories if they don't exist
mkdir -p cmd/unified/static/css
mkdir -p cmd/unified/static/js
mkdir -p cmd/unified/static/img
mkdir -p cmd/unified/templates

# Build the unified CLI
echo "Building ria..."
go build -o bin/ria cmd/unified/main.go

echo "Build complete!"
echo "Run Responsive Investment Assistant:"
echo "  ./bin/ria help                         # Show available commands"
echo "  ./bin/ria monitor                      # Monitor price movements"
echo "  ./bin/ria chat --api-key=YOUR_API_KEY  # Chat with the assistant"
echo "  ./bin/ria web --api-key=YOUR_API_KEY   # Start the web interface"
echo "  ./bin/ria ai-alerter                   # Start the AI alert analyzer"
echo ""
echo "For real market data, add these options:"
echo "  --use-real-api                         # Use real Yahoo Finance data"
echo "  --finance-api-key=YOUR_API_KEY         # For API services that require a key"
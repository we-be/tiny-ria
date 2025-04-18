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

# Create logs directory for RIA
mkdir -p /tmp/ria_logs

# Build the unified CLI
echo "Building ria..."
go build -o bin/ria cmd/unified/main.go cmd/unified/websocket_fix.go

echo "Build complete!"
echo "Run Responsive Investment Assistant:"
echo "  ./bin/ria help                         # Show available commands"
echo "  ./bin/ria monitor                      # Monitor price movements"
echo "  ./bin/ria chat -api-key=YOUR_API_KEY   # Chat with the assistant"
echo "  ./bin/ria web -api-key=YOUR_API_KEY    # Start the web interface"
echo "  ./bin/ria ai-alerter                   # Start the AI alert analyzer"
echo ""
echo "Make sure the Quotron service is running for real market data:"
echo "  cd ../quotron/cli && ./quotron start yfinance-proxy api-service"
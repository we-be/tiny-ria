#!/bin/bash

set -e

# Ensure we're in the agent directory
cd "$(dirname "$0")"

echo "Building quotron tools..."
mkdir -p bin

# Create directories if they don't exist
mkdir -p cmd/assistant
mkdir -p cmd/ai-alerter

# Build the agent CLI
echo "Building quotron-agent..."
go build -o bin/quotron-agent cmd/main.go

# Build the agent assistant
echo "Building quotron-assistant..."
go build -o bin/quotron-assistant cmd/assistant/main.go

# Build the AI alerter
echo "Building quotron-ai-alerter..."
go build -o bin/quotron-ai-alerter cmd/ai-alerter/main.go

echo "Build complete!"
echo "Run agent with: ./bin/quotron-agent --command=help"
echo "Run assistant with: ./bin/quotron-assistant --api-key=YOUR_OPENAI_API_KEY"
echo "Run AI alerter with: ./bin/quotron-ai-alerter --api-key=YOUR_OPENAI_API_KEY"
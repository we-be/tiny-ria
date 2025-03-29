#!/bin/bash

# Stop any running instances of the ria service
echo "Stopping any running ria processes..."
pkill -f "bin/ria" || true

# Build the application
echo "Building ria..."
cd "$(dirname "$0")"
./build.sh

# Get the API key from environment variables
OPENAI_KEY=${OPENAI_API_KEY:-""}
ANTHROPIC_KEY=${ANTHROPIC_API_KEY:-""}

# Choose which key to use
if [ -n "$OPENAI_KEY" ]; then
    API_KEY=$OPENAI_KEY
    echo "Using OpenAI API key"
elif [ -n "$ANTHROPIC_KEY" ]; then
    API_KEY=$ANTHROPIC_KEY
    echo "Using Anthropic API key"
else
    echo "ERROR: No API key found. Please set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable."
    exit 1
fi

# Create logs directory if it doesn't exist
mkdir -p /tmp/ria_logs

# Start the application in the background
echo "Starting ria web interface..."
nohup ./bin/ria web --api-key="$API_KEY" > /tmp/ria_logs/web.log 2>&1 &
WEB_PID=$!

# Wait a moment for the service to start
sleep 2

# Check if service is running
if ps -p $WEB_PID > /dev/null; then
    echo "RIA web interface is running on http://localhost:8090"
    echo "You can view logs at: /tmp/ria_logs/web.log"
    echo "To stop the service: pkill -f \"bin/ria\""
else
    echo "Failed to start RIA web interface. Check logs at: /tmp/ria_logs/web.log"
    exit 1
fi
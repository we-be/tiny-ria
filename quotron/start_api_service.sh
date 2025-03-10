#!/bin/bash

# Create required directories if they don't exist
mkdir -p api-service/cmd/server
mkdir -p api-service/pkg/client

echo "Starting API service and related components..."

# Check if Yahoo Finance Proxy is running
if pgrep -f "python.*yfinance_proxy.py" > /dev/null; then
    echo "Yahoo Finance proxy is already running"
else
    echo "Starting Yahoo Finance proxy..."
    cd api-scraper/scripts
    python yfinance_proxy.py > yfinance_proxy.log 2>&1 &
    cd ../../
    sleep 2
    if pgrep -f "python.*yfinance_proxy.py" > /dev/null; then
        echo "✅ Yahoo Finance proxy started successfully"
    else
        echo "❌ Failed to start Yahoo Finance proxy"
        exit 1
    fi
fi

# Build and run API service
echo "Building API service..."
cd api-service
go build -o api-service ./cmd/server
if [ $? -ne 0 ]; then
    echo "❌ Failed to build API service"
    exit 1
fi

echo "✅ API service built successfully"
echo "Starting API service..."
./api-service --port=8080 --yahoo-host=localhost --yahoo-port=5000 &
API_SERVICE_PID=$!
cd ..

echo "✅ API service started with PID: $API_SERVICE_PID"
echo "Services are now running. Press Ctrl+C to stop."

# Create a trap to handle stopping services when the script is terminated
trap "echo 'Stopping services...'; kill $API_SERVICE_PID; pkill -f 'python.*yfinance_proxy.py'; echo 'Services stopped.'; exit 0" INT TERM EXIT

# Keep the script running
while true; do sleep 1; done
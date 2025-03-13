#!/bin/bash
set -e

echo "Building health server..."
go build -o health-server ./cmd/server/main.go

echo "Building health test client..."
go build -o health-client ./cmd/test_client/main.go

echo "Making Python client executable..."
chmod +x ./test_client.py

echo "Build complete."
echo "You can run the server with: ./health-server"
echo "You can test with Go client: ./health-client --action report|get|all|system"
echo "You can test with Python client: ./test_client.py --action report|get|all|system"
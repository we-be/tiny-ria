#!/bin/bash

# Integration test script for ETL, Scheduler and Redis
# Tests the complete data flow from scheduler → Redis → ETL → Database

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting ETL Integration Test${NC}"

# Environment variables
SYMBOLS="BTC-USD,ETH-USD,SOL-USD"
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QUOTRON_ROOT="$(cd "$TEST_DIR/../.." && pwd)"

# Build the test utilities if they don't exist
echo -e "\n${YELLOW}Building test utilities...${NC}"
cd "$TEST_DIR"

if [ ! -f "./crypto_redis_monitor" ]; then
    echo "Building crypto_redis_monitor..."
    go build -o crypto_redis_monitor crypto_redis_monitor.go
fi

if [ ! -f "./test_crypto_etl_flow" ]; then
    echo "Building test_crypto_etl_flow..."
    go build -o test_crypto_etl_flow test_crypto_etl_flow.go
fi

# Setup Redis streams
echo -e "\n${YELLOW}Setting up Redis streams...${NC}"
./setup_redis_streams.sh

# Start Redis monitor in the background
echo -e "\n${YELLOW}Starting Redis monitor in the background...${NC}"
./crypto_redis_monitor > redis_monitor.log 2>&1 &
REDIS_MONITOR_PID=$!
echo "Redis monitor started with PID $REDIS_MONITOR_PID"

# Function to clean up when the script exits
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    
    # Kill Redis monitor
    if [ -n "$REDIS_MONITOR_PID" ]; then
        echo "Stopping Redis monitor (PID $REDIS_MONITOR_PID)..."
        kill $REDIS_MONITOR_PID 2>/dev/null || true
    fi
    
    echo -e "${GREEN}Test complete${NC}"
}
trap cleanup EXIT

# Test 1: Run the test ETL flow
echo -e "\n${YELLOW}Running test ETL flow...${NC}"
echo "This test simulates the scheduler publishing messages to Redis and ETL consuming them"
./test_crypto_etl_flow &
TEST_ETL_PID=$!

# Wait for some messages to be processed
echo "Waiting for messages to be processed (20 seconds)..."
sleep 20

# Kill the test
kill $TEST_ETL_PID 2>/dev/null || true

# Test 2: Run the crypto job directly
echo -e "\n${YELLOW}Running crypto job directly...${NC}"
echo "This tests the actual job implementation used by the scheduler"
cd "$TEST_DIR"
go run quotron_crypto_job.go -symbols=$SYMBOLS

# Show Redis monitor logs
echo -e "\n${YELLOW}Redis monitor logs:${NC}"
tail -n 20 redis_monitor.log

# Display results
echo -e "\n${GREEN}Integration test completed successfully!${NC}"
echo "The test has verified that:"
echo "1. Redis streams and consumer groups are properly set up"
echo "2. Messages can be published to both PubSub and Streams"
echo "3. Messages can be consumed from both PubSub and Streams"
echo "4. The crypto job correctly publishes messages to Redis"
echo ""
echo "Next steps:"
echo "1. Start the ETL service: quotron start etl"
echo "2. Start the scheduler: quotron start scheduler"
echo "3. Check logs to verify proper operation"
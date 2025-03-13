#!/bin/bash
# Real-world integration tests for the CLI
# This script actually starts services and verifies they work

set -e

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Running comprehensive CLI integration tests...${NC}"

# Navigate to the CLI directory
cd "$(dirname "$0")/.."

# Build the CLI if it doesn't exist
if [ ! -f "./quotron" ]; then
    echo -e "${YELLOW}Building CLI...${NC}"
    cd cli && ./build.sh && cd ..
fi

# Create a test configuration
echo -e "${YELLOW}Creating test configuration...${NC}"
TMP_CONFIG=$(mktemp -t quotron-config.XXXXXX.json)
./quotron --gen-config --config "$TMP_CONFIG" || { echo -e "${RED}Config generation failed${NC}"; exit 1; }
test -f "$TMP_CONFIG" || { echo -e "${RED}Config file was not created${NC}"; exit 1; }

# Check initial status - nothing should be running
echo -e "${YELLOW}Checking initial status...${NC}"
./quotron --config "$TMP_CONFIG" status 

# Start the YFinance proxy
echo -e "${YELLOW}Starting YFinance proxy...${NC}"
./quotron --config "$TMP_CONFIG" start proxy

# Verify it's running
echo -e "${YELLOW}Verifying YFinance proxy is running...${NC}"
./quotron --config "$TMP_CONFIG" status | grep -q "YFinance Proxy.*Running" || { 
    echo -e "${RED}YFinance proxy failed to start${NC}"
    
    # Check the logs
    echo -e "${YELLOW}Checking logs:${NC}"
    cat /tmp/yfinance_proxy.log
    
    exit 1
}

# Test the proxy with curl
echo -e "${YELLOW}Testing YFinance proxy with curl...${NC}"
HEALTH_OUTPUT=$(curl -s http://localhost:5000/health || echo "Connection failed")
if ! echo "$HEALTH_OUTPUT" | grep -q "status"; then
    echo -e "${RED}YFinance proxy not responding properly:${NC}"
    echo "$HEALTH_OUTPUT"
    ./quotron --config "$TMP_CONFIG" stop proxy
    exit 1
fi
echo -e "${GREEN}YFinance proxy responding with health data:${NC}"
echo "$HEALTH_OUTPUT"

# Try to get a real stock quote
echo -e "${YELLOW}Testing YFinance proxy with a real stock quote...${NC}"
QUOTE_OUTPUT=$(curl -s http://localhost:5000/quote/AAPL || echo "Connection failed")
if ! echo "$QUOTE_OUTPUT" | grep -q "AAPL"; then
    echo -e "${RED}Failed to get stock quote:${NC}"
    echo "$QUOTE_OUTPUT"
    ./quotron --config "$TMP_CONFIG" stop proxy
    exit 1
fi
echo -e "${GREEN}Successfully got AAPL stock quote from proxy${NC}"

# Stop the service
echo -e "${YELLOW}Stopping YFinance proxy...${NC}"
./quotron --config "$TMP_CONFIG" stop proxy

# Verify it's stopped
echo -e "${YELLOW}Verifying YFinance proxy is stopped...${NC}"
./quotron --config "$TMP_CONFIG" status | grep -q "YFinance Proxy.*Not running" || { 
    echo -e "${RED}YFinance proxy failed to stop${NC}"
    ./quotron --config "$TMP_CONFIG" stop proxy
    exit 1
}

# Try monitor mode with a short timeout
echo -e "${YELLOW}Testing monitor mode (short test)...${NC}"
./quotron --config "$TMP_CONFIG" --monitor start proxy &
MONITOR_PID=$!

# Give it time to start
sleep 5

# Verify it's running
echo -e "${YELLOW}Verifying YFinance proxy is running in monitor mode...${NC}"
./quotron --config "$TMP_CONFIG" status | grep -q "YFinance Proxy.*Running" || { 
    echo -e "${RED}YFinance proxy failed to start in monitor mode${NC}"
    kill $MONITOR_PID
    exit 1
}

# Kill the monitor mode process and make sure everything shuts down
echo -e "${YELLOW}Stopping monitor mode...${NC}"
kill $MONITOR_PID
sleep 2
./quotron --config "$TMP_CONFIG" stop proxy || true

# Clean up
rm -f "$TMP_CONFIG"

echo -e "${GREEN}All CLI integration tests passed!${NC}"
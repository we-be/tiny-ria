#!/bin/bash
# Run real-world tests for all consolidated components

set -e

QUOTRON_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}    RUNNING REAL-WORLD COMPONENT TESTS      ${NC}"
echo -e "${YELLOW}=============================================${NC}"

# Function to run tests for a Go package
run_go_tests() {
    local package=$1
    echo -e "${YELLOW}Running tests for $package...${NC}"
    
    cd "$QUOTRON_ROOT/$package"
    if go test -v ./...; then
        echo -e "${GREEN}Tests for $package passed!${NC}"
        return 0
    else
        echo -e "${RED}Tests for $package failed!${NC}"
        return 1
    fi
}

# Function to run integration tests for the CLI
run_cli_integration_tests() {
    echo -e "${YELLOW}Running CLI integration tests...${NC}"
    
    "$QUOTRON_ROOT/cli/test_integration.sh"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}CLI integration tests passed!${NC}"
        return 0
    else
        echo -e "${RED}CLI integration tests failed!${NC}"
        return 1
    fi
}

# Track failures
FAILURES=0

# Make sure dependencies are installed
echo -e "${YELLOW}Checking Python dependencies...${NC}"
pip list | grep -q pydantic || pip install 'pydantic==1.*'
pip list | grep -q yfinance || pip install yfinance

# Run tests for models package
echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}Testing models package (data structures)${NC}"
echo -e "${YELLOW}=============================================${NC}"
if ! run_go_tests "models"; then
    FAILURES=$((FAILURES + 1))
fi

# Run tests for providers/yahoo package
echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}Testing Yahoo provider (real API calls)${NC}"
echo -e "${YELLOW}=============================================${NC}"
if ! run_go_tests "providers/yahoo"; then
    FAILURES=$((FAILURES + 1))
fi

# Run tests for CLI package
echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}Testing CLI package (internal functions)${NC}"
echo -e "${YELLOW}=============================================${NC}"
if ! run_go_tests "cli"; then
    FAILURES=$((FAILURES + 1))
fi

# Run CLI integration tests
echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}Testing CLI integration (real services)${NC}"
echo -e "${YELLOW}=============================================${NC}"
if ! run_cli_integration_tests; then
    FAILURES=$((FAILURES + 1))
fi

# Report results
echo
echo -e "${YELLOW}=============================================${NC}"
echo -e "${YELLOW}             TEST SUMMARY                   ${NC}"
echo -e "${YELLOW}=============================================${NC}"
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo -e "${GREEN}These tests verified:${NC}"
    echo -e "${GREEN}- Data structures correctly serialize and deserialize${NC}"
    echo -e "${GREEN}- Models can be used from both Go and Python${NC}"
    echo -e "${GREEN}- Yahoo Finance provider can fetch real data${NC}"
    echo -e "${GREEN}- CLI can actually start and stop services${NC}"
    echo -e "${GREEN}- Real services respond to API requests${NC}"
else
    echo -e "${RED}$FAILURES test suites failed!${NC}"
    exit 1
fi
#!/bin/bash
# Build and manage the health service components

set -e

# Set the working directory to the health service root
cd "$(dirname "$0")"

# Command to build the health service components
build_service() {
    echo "Building health service components..."
    
    # Create bin directory if it doesn't exist
    mkdir -p bin
    
    # Build the health service and its binaries
    echo "Building health service server..."
    go build -o bin/health-server cmd/server/main.go
    
    echo "Building health service test client..."
    go build -o bin/test-client cmd/test_client/main.go
    
    echo "Health service components built successfully."
}

# Command to start the health service
start_service() {
    echo "Starting health service..."
    
    # Ensure the service is built
    if [ ! -f "bin/health-server" ]; then
        echo "Health service binary not found. Building..."
        build_service
    fi
    
    # Start the health service
    ./bin/health-server -port 8085 -db "postgres://quotron:quotron@localhost:5433/quotron?sslmode=disable"
}

# Command to test the health service
test_service() {
    echo "Testing health monitoring system..."
    
    # Ensure the test client is built
    if [ ! -f "bin/test-client" ]; then
        echo "Health service test client not found. Building..."
        build_service
    fi
    
    # Run a series of test commands
    echo "1. Checking system health..."
    ./bin/test-client -cmd system
    
    echo "2. Reporting a healthy status..."
    ./bin/test-client -cmd report -source "test-source" -name "test-client" -status "healthy"
    
    echo "3. Checking the reported status..."
    ./bin/test-client -cmd get -source "test-source" -name "test-client"
    
    echo "4. Reporting a degraded status..."
    ./bin/test-client -cmd report -source "test-source" -name "test-client" -status "degraded" -message "Performance is slow"
    
    echo "5. Checking all services..."
    ./bin/test-client -cmd all
    
    echo "Health monitoring system test complete."
}

# Display help message
show_help() {
    echo "Health Service Management"
    echo ""
    echo "Usage: ./build.sh [command]"
    echo ""
    echo "Commands:"
    echo "  build         Build the health service binaries"
    echo "  start         Start the health service"
    echo "  test          Test the health service"
    echo "  help          Show this help message"
    echo ""
}

# Main command handler
case "$1" in
    build)
        build_service
        ;;
    start)
        start_service
        ;;
    test)
        test_service
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        # Default action is to build
        build_service
        ;;
esac
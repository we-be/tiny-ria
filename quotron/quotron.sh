#!/bin/bash
# Quotron - Financial Data System CLI

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Include common functions
source "$SCRIPT_DIR/scripts/common.sh"

# Function to start all services
start_services() {
    echo "Starting Quotron services..."
    
    # Start the health service first
    echo "Starting Health Service..."
    "$SCRIPT_DIR/health/build.sh" start &
    HEALTH_PID=$!
    echo "Health Service started with PID $HEALTH_PID"
    sleep 2  # Give the health service time to start
    
    # Start the Yahoo Finance proxy
    echo "Starting Yahoo Finance Proxy..."
    cd "$SCRIPT_DIR/api-scraper/scripts" && python3 yfinance_proxy.py &
    YFINANCE_PID=$!
    echo "Yahoo Finance Proxy started with PID $YFINANCE_PID"
    sleep 2
    
    # Start the API service
    echo "Starting API Service..."
    cd "$SCRIPT_DIR/api-service/cmd/server" && go run main.go &
    API_PID=$!
    echo "API Service started with PID $API_PID"
    sleep 2
    
    # Start the dashboard
    echo "Starting Dashboard..."
    cd "$SCRIPT_DIR/dashboard" && ./launch.sh &
    DASHBOARD_PID=$!
    echo "Dashboard started with PID $DASHBOARD_PID"
    
    echo "All services started successfully."
}

# Function to stop all services
stop_services() {
    echo "Stopping Quotron services..."
    
    # Find and kill all relevant processes
    echo "Stopping Health Service..."
    pkill -f "health-server" || true
    
    echo "Stopping Yahoo Finance Proxy..."
    pkill -f "yfinance_proxy.py" || true
    
    echo "Stopping API Service..."
    pkill -f "api-service/cmd/server" || true
    
    echo "Stopping Dashboard..."
    pkill -f "dashboard.py" || true
    
    echo "All services stopped."
}

# Function to check the health of all services
check_health() {
    echo "Checking Quotron services health..."
    
    # Use the health service if available
    if curl -s -f http://localhost:8085/health/system > /dev/null; then
        echo "Using unified health service:"
        curl -s http://localhost:8085/health/system | python3 -m json.tool
    else
        echo "Health service not available, falling back to direct checks:"
        
        # Check Yahoo Finance Proxy
        if curl -s -f http://localhost:5000/health > /dev/null; then
            echo "✅ Yahoo Finance Proxy: Healthy"
        else
            echo "❌ Yahoo Finance Proxy: Not available"
        fi
        
        # Check API Service
        if curl -s -f http://localhost:8080/api/health > /dev/null; then
            echo "✅ API Service: Healthy"
        else
            echo "❌ API Service: Not available"
        fi
        
        # Check Dashboard
        if curl -s -f http://localhost:8050 > /dev/null; then
            echo "✅ Dashboard: Healthy"
        else
            echo "❌ Dashboard: Not available"
        fi
    fi
}

# Function to run tests
run_tests() {
    echo "Running Quotron tests..."
    
    # Run the test script
    "$SCRIPT_DIR/run_consolidated_tests.sh"
}

# Display help message
show_help() {
    echo "Quotron - Financial Data System CLI"
    echo ""
    echo "Usage: ./quotron.sh [command]"
    echo ""
    echo "Commands:"
    echo "  start         Start all Quotron services"
    echo "  stop          Stop all Quotron services"
    echo "  restart       Restart all Quotron services"
    echo "  status        Check the status of all services"
    echo "  health        Check the health of all services"
    echo "  test          Run all tests"
    echo "  help          Show this help message"
    echo ""
}

# Main command handler
case "$1" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        sleep 2
        start_services
        ;;
    status)
        check_service_status
        ;;
    health)
        check_health
        ;;
    test)
        run_tests
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac

exit 0
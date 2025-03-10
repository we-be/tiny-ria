#!/bin/bash

# Run all tests for the API service implementation

echo "========================================================"
echo "              API SERVICE TEST SUITE                    "
echo "========================================================"

# Change to project root
cd "$(dirname "$0")"

# Function to check if a command was successful
check_result() {
    if [ $? -ne 0 ]; then
        echo "❌ $1 failed"
        exit 1
    else
        echo "✅ $1 succeeded"
    fi
}

# Step 1: Start prerequisite services
echo "[1/6] Starting Yahoo Finance proxy and API service..."
./start_api_service.sh &
START_PID=$!

# Wait for services to start
echo "Waiting 10 seconds for services to start..."
sleep 10

# Step 2: Run basic API service tests
echo "[2/6] Running API service tests..."
./test_api_service.sh
check_result "API service tests"

# Step 3: Test database connectivity and tables
echo "[3/6] Testing database migrations..."
PGPASSWORD=postgres psql -h localhost -p 5433 -U postgres -d quotron -c "\dt" | grep -q "stock_quotes"
check_result "Database migration check"

# Step 4: Test Yahoo Finance directly
echo "[4/6] Testing Yahoo Finance proxy directly..."
cd api-scraper/scripts
python -c "
import requests
try:
    response = requests.get('http://localhost:5000/quote/AAPL')
    if response.status_code == 200 and 'AAPL' in response.text:
        print('Yahoo Finance proxy is working correctly')
        exit(0)
    else:
        print(f'Unexpected response: {response.status_code}, {response.text[:100]}')
        exit(1)
except Exception as e:
    print(f'Error: {e}')
    exit(1)
"
check_result "Yahoo Finance proxy test"
cd ../../

# Step 5: Run integration tests for the API
echo "[5/6] Running integration tests..."
cd tests
python yahoo_finance_test.py
check_result "Yahoo Finance integration test"
cd ..

# Step 6: Run scheduler tests with API service
echo "[6/6] Running scheduler tests with API service..."
cd scheduler
go run cmd/scheduler/main.go --config ../scheduler-config.json --use-api-service --api-host localhost --api-port 8080 --run-job stock_quotes
check_result "Scheduler stock_quotes test"
cd ..

# Clean up
echo "Stopping services..."
kill $START_PID
pkill -f "python.*yfinance_proxy.py"

echo "========================================================"
echo "              ALL TESTS PASSED! 🎉                      "
echo "========================================================"
#!/bin/bash

echo "Testing API service functionality..."

# Check if API service is running
if curl -s http://localhost:8080/api/health > /dev/null; then
    echo "✅ API service is running"
else
    echo "❌ API service is not running. Please start it first with ./start_api_service.sh"
    exit 1
fi

# Test stock quote endpoint
echo "Testing stock quote endpoint..."
curl -s http://localhost:8080/api/quote/AAPL | python -m json.tool
if [ $? -ne 0 ]; then
    echo "❌ Failed to fetch stock quote"
    exit 1
fi
echo "✅ Stock quote endpoint is working"

# Test market index endpoint
echo "Testing market index endpoint..."
curl -s http://localhost:8080/api/index/SPY | python -m json.tool
if [ $? -ne 0 ]; then
    echo "❌ Failed to fetch market index data"
    exit 1
fi
echo "✅ Market index endpoint is working"

# Test scheduler with API service
echo "Testing scheduler with API service..."
cd scheduler
go run cmd/scheduler/main.go --config ../scheduler-config.json --use-api-service --api-host localhost --api-port 8080 --run-job stock_quotes
if [ $? -ne 0 ]; then
    echo "❌ Scheduler test failed"
    exit 1
fi
echo "✅ Scheduler test completed successfully"

echo "All tests completed successfully!"
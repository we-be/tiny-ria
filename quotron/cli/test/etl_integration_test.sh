#!/bin/bash

# Script to test the ETL service with both stock and crypto quotes

# Start Redis monitor in the background
./crypto_redis_monitor --duration 180 > redis_monitor.log 2>&1 &
MONITOR_PID=$!
echo "Started Redis monitor (PID: $MONITOR_PID)"

# Wait for the monitor to initialize
sleep 2

# Run the crypto test job to publish to Redis
echo "Running crypto test job to publish data to Redis..."
./test_crypto_etl --symbol "BTC-USD,ETH-USD" > crypto_test.log 2>&1
echo "Published crypto data to Redis"

# Show the Redis monitor log
echo "Redis Monitor Output:"
cat redis_monitor.log

# Cleanup
kill $MONITOR_PID 2>/dev/null
echo "Test completed"
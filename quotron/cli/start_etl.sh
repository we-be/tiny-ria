#!/bin/bash
# Start the ETL service

# Change to the script directory
cd "$(dirname "$0")"

# Verify ETL executable exists
ETL_EXEC="./cmd/etl/etl"
if [ ! -x "$ETL_EXEC" ]; then
    echo "ETL executable not found or not executable: $ETL_EXEC"
    echo "Building ETL..."
    
    # Try to build it
    cd cmd/etl
    go build -o etl main.go
    if [ $? -ne 0 ]; then
        echo "Failed to build ETL executable"
        exit 1
    fi
    cd ../..
fi

echo "Starting ETL service..."
nohup ./cmd/etl/etl -start -redis=localhost:6379 -dbhost=localhost -dbport=5432 -dbname=quotron -dbuser=quotron -dbpass=quotron -workers=2 >> /tmp/etl_service.log 2>&1 &

# Store the PID
PID=$!
echo "ETL service started with PID: $PID"

# Create the pid file
mkdir -p /tmp/quotron
echo $PID > /tmp/quotron/etl_service.pid
echo $PID > .etl_service.pid

# Wait a moment to make sure it's started properly
sleep 2
if ps -p $PID > /dev/null; then
    echo "ETL service running successfully"
else
    echo "WARNING: ETL service may have failed to start"
    cat /tmp/etl_service.log
fi

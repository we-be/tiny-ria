#!/bin/bash
# Common functions for Quotron scripts

# Function to check if a service is running
is_service_running() {
    local process_pattern=$1
    pgrep -f "$process_pattern" > /dev/null
    return $?
}

# Function to check the status of all services
check_service_status() {
    echo "Checking Quotron services status..."
    
    # Check Health Service
    if is_service_running "health-server"; then
        echo "✅ Health Service: Running"
    else
        echo "❌ Health Service: Not running"
    fi
    
    # Check Yahoo Finance Proxy
    if is_service_running "yfinance_proxy.py"; then
        echo "✅ Yahoo Finance Proxy: Running"
    else
        echo "❌ Yahoo Finance Proxy: Not running"
    fi
    
    # Check API Service
    if is_service_running "api-service/cmd/server"; then
        echo "✅ API Service: Running"
    else
        echo "❌ API Service: Not running"
    fi
    
    # Check Dashboard
    if is_service_running "dashboard.py"; then
        echo "✅ Dashboard: Running"
    else
        echo "❌ Dashboard: Not running"
    fi
}

# Function to check if a port is in use
is_port_in_use() {
    local port=$1
    netstat -tuln | grep ":$port " > /dev/null
    return $?
}

# Function to wait for a service to be available
wait_for_service() {
    local service_name=$1
    local url=$2
    local max_attempts=$3
    local attempt=1
    
    echo "Waiting for $service_name to be available..."
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url" > /dev/null; then
            echo "$service_name is now available."
            return 0
        fi
        
        echo "Attempt $attempt/$max_attempts: $service_name not yet available, waiting..."
        sleep 2
        ((attempt++))
    done
    
    echo "Failed to connect to $service_name after $max_attempts attempts."
    return 1
}
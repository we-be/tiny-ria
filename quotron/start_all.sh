#!/bin/bash
# Master script to start all Quotron services

# Load environment variables from .env file
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
else
    echo "Warning: .env file not found! Using default values."
fi

echo "Stopping any existing services..."
pkill -f 'yfinance_proxy.py|scheduler|python.*dashboard.py'
sleep 3

echo "Starting data services..."
bash /home/hunter/Desktop/tiny-ria/quotron/start_services.sh

# Ensure the compiled API scraper binary exists
cd /home/hunter/Desktop/tiny-ria/quotron/api-scraper 
if [ ! -f api-scraper ] || [ ! -x api-scraper ]; then
    echo "Rebuilding the API scraper binary..."
    go build -o api-scraper ./cmd/main/main.go
    chmod +x api-scraper
fi

echo "Starting dashboard..."
bash /home/hunter/Desktop/tiny-ria/quotron/restart_dashboard.sh

echo "All services started successfully!"
echo
echo "Dashboard: http://localhost:8501"
echo "YFinance proxy: http://localhost:5000/health"
echo
echo "Logs:"
echo "- Dashboard: /tmp/dashboard.log"
echo "- YFinance proxy: /tmp/yfinance_proxy.log"
echo "- Scheduler: /tmp/scheduler.log"
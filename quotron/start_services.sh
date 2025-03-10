#!/bin/bash
# Script to start all required Quotron services

echo "Starting YFinance Proxy..."
cd /home/hunter/Desktop/tiny-ria/quotron/api-scraper
pkill -f yfinance_proxy.py  # Kill any existing instances
python3 scripts/yfinance_proxy.py --host localhost --port 5000 > /tmp/yfinance_proxy.log 2>&1 &
PROXY_PID=$!
echo "YFinance Proxy started with PID $PROXY_PID"

# Set environment variable for dashboard to find the proxy
export YFINANCE_PROXY_URL=http://localhost:5000

# Wait for proxy to initialize
sleep 5

echo "Starting Scheduler..."
cd /home/hunter/Desktop/tiny-ria/quotron/scheduler
export ALPHA_VANTAGE_API_KEY="Q3R4E9KFVLXOWEXN"  # Using real Alpha Vantage API key
./scheduler -api-scraper /home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper > /tmp/scheduler.log 2>&1 &
SCHEDULER_PID=$!
echo "Scheduler started with PID $SCHEDULER_PID"

echo "Services started successfully. Check logs at:"
echo "- /tmp/yfinance_proxy.log"
echo "- /tmp/scheduler.log"

echo "To stop services: pkill -f 'yfinance_proxy.py|scheduler'"
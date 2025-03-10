#!/bin/bash
# Script to restart the Quotron dashboard

# Load environment variables from .env file
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
else
    echo "Warning: .env file not found! Using default values."
    # Set default environment variables
    export YFINANCE_PROXY_URL=http://localhost:5000
    export DB_HOST=localhost
    export DB_PORT=5432
    export DB_NAME=quotron
    export DB_USER=quotron
    export DB_PASSWORD=quotron
    export SCHEDULER_PATH=/home/hunter/Desktop/tiny-ria/quotron/scheduler
fi

# Make sure API_SCRAPER_PATH is set
export API_SCRAPER_PATH=/home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper

# Kill any existing dashboard instances
pkill -f "python.*dashboard.py"
sleep 2

echo "Starting Quotron dashboard..."
cd /home/hunter/Desktop/tiny-ria/quotron/dashboard
python dashboard.py > /tmp/dashboard.log 2>&1 &
DASHBOARD_PID=$!
echo "Dashboard started with PID $DASHBOARD_PID"
echo "Dashboard available at http://localhost:8501"
echo "Monitor logs with: tail -f /tmp/dashboard.log"
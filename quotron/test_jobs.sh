#!/bin/bash
# Test script to verify job execution in the scheduler

# Load environment variables from .env file
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
else
    echo "Warning: .env file not found! Using default values."
fi

echo "============================================="
echo "Testing Stock Quotes Job"
echo "============================================="
cd /home/hunter/Desktop/tiny-ria/quotron/scheduler
./scheduler -run-job stock_quotes -api-scraper /home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper

echo 
echo "============================================="
echo "Testing Market Indices Job"
echo "============================================="
cd /home/hunter/Desktop/tiny-ria/quotron/scheduler
./scheduler -run-job market_indices -api-scraper /home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper

echo
echo "Job testing complete!"
#!/bin/bash
# Script to scrape S&P 500 data and import it into the database

set -e

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
QUOTRON_DIR="$(dirname "$SCRIPT_DIR")"
PYTHON_SCRAPER_DIR="$QUOTRON_DIR/browser-scraper/src"
ETL_DIR="$QUOTRON_DIR/etl"

# Ensure output directory exists
OUTPUT_DIR="$QUOTRON_DIR/browser-scraper/output"
mkdir -p "$OUTPUT_DIR"

echo "=== Starting S&P 500 Model Import Process ==="

# Step 1: Run the Python scraper to fetch S&P 500 data
echo "Step 1: Scraping S&P 500 data from slickcharts.com..."
cd "$PYTHON_SCRAPER_DIR"
python_output=$(python3 sp500_scraper.py | tail -n 1)
if [ $? -ne 0 ]; then
  echo "Error: Failed to scrape S&P 500 data"
  exit 1
fi

# Get the output file path from the Python script
json_file="$python_output"
echo "Python script output file: $json_file"
if [ ! -f "$json_file" ]; then
  echo "Error: S&P 500 data file not found at $json_file"
  exit 1
fi

echo "Successfully scraped S&P 500 data to $json_file"

# Step 2: Import the data into the database using the Go CLI
echo "Step 2: Importing S&P 500 model into database..."
cd "$ETL_DIR"
go run cmd/modelcli/main.go -import -file="$json_file"
if [ $? -ne 0 ]; then
  echo "Error: Failed to import S&P 500 model into database"
  exit 1
fi

echo "=== S&P 500 Model Import Process Completed Successfully ==="
#!/bin/bash
# Simple script to run the YFinance proxy server

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR"

# Environment variables
export PYTHONUNBUFFERED=1
export FLASK_ENV=production

# Use python from virtualenv if it exists
if [ -f "../../.venv/bin/python" ]; then
  PYTHON="../../.venv/bin/python"
else
  PYTHON="python3"
fi

# Log to stdout and a log file
LOG_FILE="/tmp/yfinance_proxy.log"
echo "=== Starting YFinance Proxy $(date) ===" | tee -a "$LOG_FILE"

# Run the proxy server
exec "$PYTHON" "$SCRIPT_DIR/yfinance_proxy.py" "$@" 2>&1 | tee -a "$LOG_FILE"
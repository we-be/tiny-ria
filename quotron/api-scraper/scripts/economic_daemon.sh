#!/bin/bash
#
# Economic Factors Daemon Script
#
# This script manages the Economic Factors proxy, allowing it to run in the background
# and restart automatically if it crashes.
#
# Usage:
#   ./economic_daemon.sh           # Start the daemon with default settings
#   ./economic_daemon.sh --port 5003  # Start with a custom port
#   ./economic_daemon.sh stop      # Stop the daemon
#   ./economic_daemon.sh status    # Check daemon status
#

# Default settings
HOST="localhost"
PORT=5002
PID_FILE="/tmp/economic_proxy.pid"
LOG_FILE="/tmp/economic_proxy.log"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROXY_SCRIPT="${SCRIPT_DIR}/economic_proxy.py"

# Process command line arguments
if [ "$1" == "stop" ]; then
    echo "Stopping Economic Factors proxy daemon..."
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null; then
            kill "$PID"
            echo "Proxy stopped (PID: $PID)"
        else
            echo "Proxy not running (stale PID file)"
        fi
        rm -f "$PID_FILE"
    else
        echo "Proxy not running (no PID file)"
    fi
    exit 0
elif [ "$1" == "status" ]; then
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null; then
            echo "Economic Factors proxy is running (PID: $PID)"
            echo "Listening on $HOST:$PORT"
            echo "Log file: $LOG_FILE"
        else
            echo "Economic Factors proxy is not running (stale PID file)"
            rm -f "$PID_FILE"
        fi
    else
        echo "Economic Factors proxy is not running"
    fi
    exit 0
else
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --host)
                HOST="$2"
                shift 2
                ;;
            --port)
                PORT="$2"
                shift 2
                ;;
            *)
                echo "Unknown option: $1"
                echo "Usage: $0 [--host HOST] [--port PORT] [stop|status]"
                exit 1
                ;;
        esac
    done
fi

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p "$PID" > /dev/null; then
        echo "Economic Factors proxy already running with PID: $PID"
        exit 0
    else
        echo "Removing stale PID file"
        rm -f "$PID_FILE"
    fi
fi

# Check if proxy script exists
if [ ! -f "$PROXY_SCRIPT" ]; then
    echo "Error: Proxy script not found at $PROXY_SCRIPT"
    exit 1
fi

# Ensure python3 is available
if ! command -v python3 &> /dev/null; then
    echo "Error: python3 is required but not installed"
    exit 1
fi

# Create virtual environment if it doesn't exist
if [ ! -d "${SCRIPT_DIR}/venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv "${SCRIPT_DIR}/venv"
    
    echo "Installing dependencies..."
    "${SCRIPT_DIR}/venv/bin/pip" install -r "${SCRIPT_DIR}/requirements.txt"
    
    # Install fredapi and pandas for real data
    echo "Installing FRED API client and pandas..."
    "${SCRIPT_DIR}/venv/bin/pip" install fredapi pandas
fi

# Start daemon process
echo "Starting Economic Factors proxy daemon..."
echo "Host: $HOST"
echo "Port: $PORT"
echo "PID file: $PID_FILE"
echo "Log file: $LOG_FILE"

# Set the FRED API key from environment if available
if [ -n "$FRED_API_KEY" ]; then
    echo "Using FRED API key from environment variable"
else
    echo "FRED API key not found in environment. Will use fallback data."
    echo "Set the FRED_API_KEY environment variable to use real data:"
    echo "    export FRED_API_KEY=your_api_key"
fi

# Launch with nohup to keep running after terminal closes using venv python
nohup "${SCRIPT_DIR}/venv/bin/python" "$PROXY_SCRIPT" --host "$HOST" --port "$PORT" > "$LOG_FILE" 2>&1 &

# Save PID to file
echo $! > "$PID_FILE"
echo "Economic Factors proxy started with PID: $!"
echo "Use '$0 stop' to stop the daemon"
echo "Use '$0 status' to check status"
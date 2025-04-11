#!/bin/bash
# Daemonization script for Google Trends proxy
set -e

# Script configuration
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
LOG_FILE="/tmp/gtrends_proxy.log"
PID_FILE="/tmp/gtrends_proxy.pid"
PYTHON_SCRIPT="$SCRIPT_DIR/gtrends_proxy.py"

# Determine Python executable - first try venv in scripts dir, then parent, then fallback to python3
if [ -f "$SCRIPT_DIR/venv/bin/python" ]; then
  PYTHON="$SCRIPT_DIR/venv/bin/python"
elif [ -f "$SCRIPT_DIR/../../.venv/bin/python" ]; then
  PYTHON="$SCRIPT_DIR/../../.venv/bin/python"
else
  PYTHON="python3"
fi

# Command line arguments with defaults
HOST="localhost"
PORT="5001"  # Different default port from yfinance proxy

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
      # Unknown option
      shift
      ;;
  esac
done

# Function to check if proxy is already running
is_running() {
  if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
      return 0
    fi
  fi
  return 1
}

# Function to stop the proxy
stop_proxy() {
  echo "Stopping Google Trends proxy..."
  if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
      kill "$PID"
      echo "Process with PID $PID killed"
      sleep 2
      # Force kill if still running
      if kill -0 "$PID" 2>/dev/null; then
        kill -9 "$PID"
        echo "Process with PID $PID forcefully killed"
      fi
    else
      echo "Process with PID $PID not running"
    fi
    rm -f "$PID_FILE"
  else
    echo "PID file not found, checking for running process..."
    # Try to find the process by name
    PIDS=$(pgrep -f "python.*gtrends_proxy.py")
    if [ -n "$PIDS" ]; then
      echo "Found running processes: $PIDS"
      for PID in $PIDS; do
        kill "$PID" 2>/dev/null || true
        echo "Killed process with PID $PID"
      done
    fi
  fi
  echo "Google Trends proxy stopped"
  return 0
}

# Handle stop command first - priority over everything else
if [ "$1" = "stop" ]; then
  # Always attempt to kill any running process
  PIDS=$(pgrep -f "python.*gtrends_proxy.py")
  if [ -n "$PIDS" ]; then
    echo "Found running processes: $PIDS"
    for PID in $PIDS; do
      kill -9 "$PID" 2>/dev/null || true
      echo "Killed process with PID $PID"
    done
  fi
  # Also run the normal stop function
  stop_proxy
  echo "All Google Trends proxy processes stopped"
  exit 0
fi

# Then check if proxy is already running
if is_running; then
  echo "Google Trends proxy is already running with PID $(cat $PID_FILE)"
  
  # Check if it's responding
  if curl -s "http://$HOST:$PORT" >/dev/null; then
    echo "Service is responding at http://$HOST:$PORT"
    exit 0
  else
    echo "Process is running but not responding."
    echo "Consider stopping it first: '$0 stop'"
    exit 1
  fi
fi

# Start the daemon
echo "Starting Google Trends proxy daemon on $HOST:$PORT..."
echo "Logs will be written to $LOG_FILE"

# Clean up old log
> "$LOG_FILE"

# Start the proxy in the background with proper output redirection
cd "$SCRIPT_DIR"
nohup "$PYTHON" "$PYTHON_SCRIPT" --host "$HOST" --port "$PORT" > "$LOG_FILE" 2>&1 &

# Save the PID
echo $! > "$PID_FILE"
echo "Google Trends proxy started with PID $!"

# Verify process is running
sleep 2
if is_running; then
  echo "Daemon successfully started"
  
  # Wait for proxy to be responsive (up to 15 seconds)
  echo "Waiting for proxy to respond..."
  for i in {1..15}; do
    if curl -s "http://$HOST:$PORT" >/dev/null; then
      echo "Google Trends proxy is now available at http://$HOST:$PORT"
      exit 0
    fi
    echo -n "."
    sleep 1
  done
  
  echo ""
  echo "Warning: Proxy started but not responding. Check logs at $LOG_FILE"
  tail -n 20 "$LOG_FILE"
  exit 2
else
  echo "Failed to start daemon, check logs at $LOG_FILE"
  tail -n 20 "$LOG_FILE"
  exit 1
fi
#!/bin/bash
# Daemonization script for YFinance proxy
set -e

# Script configuration
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
LOG_FILE="/tmp/yfinance_proxy.log"
PID_FILE="/tmp/yfinance_proxy.pid"
PYTHON_SCRIPT="$SCRIPT_DIR/yfinance_proxy.py"

# Determine Python executable
if [ -f "$SCRIPT_DIR/../../.venv/bin/python" ]; then
  PYTHON="$SCRIPT_DIR/../../.venv/bin/python"
else
  PYTHON="python3"
fi

# Command line arguments with defaults
HOST="localhost"
PORT="5000"

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
  echo "Stopping YFinance proxy..."
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
    PIDS=$(pgrep -f "python.*yfinance_proxy.py")
    if [ -n "$PIDS" ]; then
      echo "Found running processes: $PIDS"
      for PID in $PIDS; do
        kill "$PID" 2>/dev/null || true
        echo "Killed process with PID $PID"
      done
    fi
  fi
  echo "YFinance proxy stopped"
  return 0
}

# Check if proxy is already running
if is_running; then
  echo "YFinance proxy is already running with PID $(cat $PID_FILE)"
  echo "Use '$0 stop' to stop it first"
  exit 1
fi

# Handle stop command
if [ "$1" = "stop" ]; then
  stop_proxy
  exit 0
fi

# Start the daemon
echo "Starting YFinance proxy daemon on $HOST:$PORT..."
echo "Logs will be written to $LOG_FILE"

# Clean up old log
> "$LOG_FILE"

# Start the proxy in the background with proper output redirection
cd "$SCRIPT_DIR"
nohup "$PYTHON" "$PYTHON_SCRIPT" --host "$HOST" --port "$PORT" > "$LOG_FILE" 2>&1 &

# Save the PID
echo $! > "$PID_FILE"
echo "YFinance proxy started with PID $!"

# Verify process is running
sleep 2
if is_running; then
  echo "Daemon successfully started"
  
  # Wait for proxy to be responsive (up to 15 seconds)
  echo "Waiting for proxy to respond..."
  for i in {1..15}; do
    if curl -s "http://$HOST:$PORT" >/dev/null; then
      echo "YFinance proxy is now available at http://$HOST:$PORT"
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
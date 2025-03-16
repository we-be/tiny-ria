#!/bin/bash
# Enhanced script to forcefully kill any yfinance proxy processes
# Added special handling for macOS quirks

echo "Forcefully stopping all YFinance proxy processes..."

# First approach: Use pgrep to find processes
PIDS=$(pgrep -f "python.*yfinance_proxy.py" 2>/dev/null)
if [ -n "$PIDS" ]; then
  echo "Found running processes: $PIDS"
  for PID in $PIDS; do
    echo "Killing process with PID $PID..."
    kill -9 $PID 2>/dev/null
    if [ $? -eq 0 ]; then
      echo "Successfully killed process $PID"
    else
      echo "Failed to kill process $PID"
    fi
  done
else
  echo "No processes found with pgrep"
fi

# Second approach: Use ps | grep for macOS compatibility
echo "Checking for additional processes using ps..."
# Use ps with special handling for BSD (macOS) and GNU (Linux) options
PS_CMD="ps aux"
if [[ "$(uname)" == "Darwin" ]]; then
  PS_CMD="ps -ef"
fi

ADDITIONAL_PIDS=$($PS_CMD | grep "python.*yfinance_proxy.py" | grep -v grep | awk '{print $2}')
if [ -n "$ADDITIONAL_PIDS" ]; then
  echo "Found additional processes with ps: $ADDITIONAL_PIDS"
  for PID in $ADDITIONAL_PIDS; do
    echo "Killing process with PID $PID..."
    kill -9 $PID 2>/dev/null
    if [ $? -eq 0 ]; then
      echo "Successfully killed process $PID"
    else
      echo "Failed to kill process $PID"
    fi
  done
else
  echo "No additional processes found with ps"
fi

# Third approach: Try to check if port 5000 is in use (most likely by yfinance proxy)
if command -v lsof >/dev/null 2>&1; then
  echo "Checking for processes using port 5000..."
  PORT_PIDS=$(lsof -ti:5000 2>/dev/null)
  if [ -n "$PORT_PIDS" ]; then
    echo "Found processes using port 5000: $PORT_PIDS"
    for PID in $PORT_PIDS; do
      echo "Killing process with PID $PID..."
      kill -9 $PID 2>/dev/null
      if [ $? -eq 0 ]; then
        echo "Successfully killed process $PID"
      else
        echo "Failed to kill process $PID"
      fi
    done
  else
    echo "No processes found using port 5000"
  fi
fi

# Cleanup PID files (try multiple locations)
for PID_FILE in "/tmp/yfinance_proxy.pid" "./.yfinance_proxy.pid" "../.yfinance_proxy.pid"; do
  if [ -f "$PID_FILE" ]; then
    echo "Removing PID file: $PID_FILE"
    rm -f "$PID_FILE"
  fi
done

# On macOS, try the specific local PID file location
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PARENT_DIR="$(dirname "$SCRIPT_DIR")"
LOCAL_PID_FILE="$PARENT_DIR/.yfinance_proxy.pid"
if [ -f "$LOCAL_PID_FILE" ]; then
  echo "Removing local PID file: $LOCAL_PID_FILE"
  rm -f "$LOCAL_PID_FILE"
fi

echo "All YFinance proxy processes should now be stopped"
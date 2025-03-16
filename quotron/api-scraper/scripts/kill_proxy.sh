#!/bin/bash
# Simple script to forcefully kill any yfinance proxy processes

echo "Forcefully stopping all YFinance proxy processes..."

# Find and kill all python processes running yfinance_proxy.py
PIDS=$(pgrep -f "python.*yfinance_proxy.py")
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
  echo "All YFinance proxy processes stopped"
else
  echo "No running YFinance proxy processes found"
fi

# Clean up PID file
PID_FILE="/tmp/yfinance_proxy.pid"
if [ -f "$PID_FILE" ]; then
  echo "Removing PID file: $PID_FILE"
  rm -f "$PID_FILE"
fi

echo "Done"
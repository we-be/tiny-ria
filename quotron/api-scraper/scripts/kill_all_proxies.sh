#!/bin/bash
# Script to forcefully kill all proxy processes (YFinance, Google Trends, and Economic Factors)

# Kill YFinance proxies
echo "=== Stopping YFinance proxy processes ==="
YFINANCE_PIDS=$(pgrep -f "python.*yfinance_proxy.py" 2>/dev/null)
if [ -n "$YFINANCE_PIDS" ]; then
  echo "Found running YFinance processes: $YFINANCE_PIDS"
  for PID in $YFINANCE_PIDS; do
    echo "Killing process with PID $PID..."
    kill -9 $PID 2>/dev/null
    if [ $? -eq 0 ]; then
      echo "Successfully killed process $PID"
    else
      echo "Failed to kill process $PID"
    fi
  done
else
  echo "No YFinance processes found"
fi

# Kill Google Trends proxies
echo "=== Stopping Google Trends proxy processes ==="
GTRENDS_PIDS=$(pgrep -f "python.*gtrends_proxy.py" 2>/dev/null)
if [ -n "$GTRENDS_PIDS" ]; then
  echo "Found running Google Trends processes: $GTRENDS_PIDS"
  for PID in $GTRENDS_PIDS; do
    echo "Killing process with PID $PID..."
    kill -9 $PID 2>/dev/null
    if [ $? -eq 0 ]; then
      echo "Successfully killed process $PID"
    else
      echo "Failed to kill process $PID"
    fi
  done
else
  echo "No Google Trends processes found"
fi

# Kill Economic Factors proxies
echo "=== Stopping Economic Factors proxy processes ==="
ECONOMIC_PIDS=$(pgrep -f "python.*economic_proxy.py" 2>/dev/null)
if [ -n "$ECONOMIC_PIDS" ]; then
  echo "Found running Economic Factors processes: $ECONOMIC_PIDS"
  for PID in $ECONOMIC_PIDS; do
    echo "Killing process with PID $PID..."
    kill -9 $PID 2>/dev/null
    if [ $? -eq 0 ]; then
      echo "Successfully killed process $PID"
    else
      echo "Failed to kill process $PID"
    fi
  done
else
  echo "No Economic Factors processes found"
fi

# Check for processes on the ports
if command -v lsof >/dev/null 2>&1; then
  echo "=== Checking for processes on proxy ports ==="
  
  # Check YFinance port (5000)
  echo "Checking for processes using port 5000 (YFinance)..."
  PORT_5000_PIDS=$(lsof -ti:5000 2>/dev/null)
  if [ -n "$PORT_5000_PIDS" ]; then
    echo "Found processes using port 5000: $PORT_5000_PIDS"
    for PID in $PORT_5000_PIDS; do
      echo "Killing process with PID $PID..."
      kill -9 $PID 2>/dev/null
    done
  else
    echo "No processes found using port 5000"
  fi
  
  # Check Google Trends port (5001)
  echo "Checking for processes using port 5001 (Google Trends)..."
  PORT_5001_PIDS=$(lsof -ti:5001 2>/dev/null)
  if [ -n "$PORT_5001_PIDS" ]; then
    echo "Found processes using port 5001: $PORT_5001_PIDS"
    for PID in $PORT_5001_PIDS; do
      echo "Killing process with PID $PID..."
      kill -9 $PID 2>/dev/null
    done
  else
    echo "No processes found using port 5001"
  fi
  
  # Check Economic Factors port (5002)
  echo "Checking for processes using port 5002 (Economic Factors)..."
  PORT_5002_PIDS=$(lsof -ti:5002 2>/dev/null)
  if [ -n "$PORT_5002_PIDS" ]; then
    echo "Found processes using port 5002: $PORT_5002_PIDS"
    for PID in $PORT_5002_PIDS; do
      echo "Killing process with PID $PID..."
      kill -9 $PID 2>/dev/null
    done
  else
    echo "No processes found using port 5002"
  fi
fi

# Cleanup PID files
echo "=== Cleaning up PID files ==="
for PID_FILE in "/tmp/yfinance_proxy.pid" "/tmp/gtrends_proxy.pid" "/tmp/economic_proxy.pid" "./.yfinance_proxy.pid" "./.gtrends_proxy.pid" "./.economic_proxy.pid"; do
  if [ -f "$PID_FILE" ]; then
    echo "Removing PID file: $PID_FILE"
    rm -f "$PID_FILE"
  fi
done

echo "All proxy processes should now be stopped"
#!/bin/bash
# Helper script to start the YFinance proxy

VENV_PATH="$1"
SCRIPT_PATH="$2"
HOST="$3"
PORT="$4"
LOG_FILE="$5"
PID_FILE="$6"

# Use virtualenv python if available
if [ -d "$VENV_PATH" ]; then
    PYTHON="$VENV_PATH/bin/python"
else
    PYTHON="python"
fi

# Make script executable
chmod +x "$SCRIPT_PATH"

# Start the proxy with nohup
cd "$(dirname "$SCRIPT_PATH")"
nohup "$PYTHON" -u "$SCRIPT_PATH" --host "$HOST" --port "$PORT" > "$LOG_FILE" 2>&1 &

# Save PID
echo $! > "$PID_FILE"

# Return the PID
echo $!
#!/bin/bash
# This is a wrapper script for the CLI's scheduler functionality
# It uses the Quotron CLI to start the scheduler

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
QUOTRON_ROOT="$(dirname "$SCRIPT_DIR")"
CLI_PATH="$QUOTRON_ROOT/cli/quotron"

# Log the start
echo "$(date -Iseconds) Starting scheduler via CLI" >> "$SCRIPT_DIR/scheduler.log"

# Check if CLI is built
if [ ! -f "$CLI_PATH" ]; then
    echo "Building CLI first..." >> "$SCRIPT_DIR/scheduler.log"
    cd "$QUOTRON_ROOT/cli" && go build -o quotron cmd/main/main.go
    if [ $? -ne 0 ]; then
        echo "Failed to build CLI" >> "$SCRIPT_DIR/scheduler.log"
        exit 1
    fi
fi

# Start the scheduler using the CLI
"$CLI_PATH" start scheduler

# Start the heartbeat loop
SCHEDULER_PID=$(cat "$QUOTRON_ROOT/scheduler/.scheduler.pid" 2>/dev/null)
if [ -n "$SCHEDULER_PID" ]; then
    while kill -0 $SCHEDULER_PID 2>/dev/null; do
        echo $(date -Iseconds) > "$SCRIPT_DIR/scheduler_heartbeat"
        sleep 5
    done
fi
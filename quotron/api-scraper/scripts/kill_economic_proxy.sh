#!/bin/bash
#
# Kill Economic Factors Proxy
#
# This script finds and kills all running instances of the Economic Factors proxy.
#

echo "Searching for Economic Factors proxy processes..."

# Find all python processes running economic_proxy.py
PIDS=$(ps -ef | grep "python.*economic_proxy.py" | grep -v grep | awk '{print $2}')

if [ -z "$PIDS" ]; then
    echo "No Economic Factors proxy processes found."
else
    for PID in $PIDS; do
        echo "Killing Economic Factors proxy process with PID: $PID"
        kill $PID
    done
    echo "All Economic Factors proxy processes have been terminated."
fi

# Remove PID file if it exists
if [ -f /tmp/economic_proxy.pid ]; then
    rm /tmp/economic_proxy.pid
    echo "Removed PID file"
fi
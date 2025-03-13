#!/bin/bash
# Stop all Quotron services

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Use the quotron.sh script to stop all services
"$SCRIPT_DIR/quotron.sh" stop
#!/bin/bash
# Stop all Quotron services

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Use the Quotron CLI to stop all services
"$SCRIPT_DIR/quotron" stop
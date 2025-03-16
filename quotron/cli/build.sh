#!/bin/bash
# Build and install the Quotron CLI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building Quotron CLI..."
go build -o quotron ./cmd/main

echo "Installing to ../quotron binary..."
cp quotron ..

# Update README only if we're in a CI environment or if UPDATE_README=1
# This ensures the badge is only updated during CI builds or when explicitly requested
# To manually update the README: UPDATE_README=1 ./build.sh
if [ -n "$CI" ] || [ "$UPDATE_README" = "1" ]; then
  # Get current git commit hash
  COMMIT_HASH=$(git rev-parse --short HEAD)

  # Update README.md with current CLI help output and badge
  if [ -f "README.md" ]; then
    echo "Updating README.md..."
    
    # Get current CLI help output
    CLI_HELP=$(./quotron help)
    
    # Create temporary file with updated content
    cat > README.md.new << EOF
# Quotron CLI

[![CLI Build:${COMMIT_HASH}](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/cli-release.yml?label=CLI%20Build%3A${COMMIT_HASH}&logo=go)](https://github.com/we-be/tiny-ria/actions/workflows/cli-release.yml)

This CLI provides a unified interface for managing all Quotron services and operations.

## Overview

The Quotron CLI replaces the various bash scripts that were previously used to manage the Quotron system, consolidating them into a single, consistent interface. It provides commands for starting and stopping services, running tests, and importing data.

## Installation

To build and install the CLI, run:

\`\`\`bash
cd cli
./build.sh
\`\`\`

This will create a \`quotron\` binary in the parent directory.

## Usage

<!-- CLI_HELP_START -->
\`\`\`
${CLI_HELP}
\`\`\`
<!-- CLI_HELP_END -->

## Examples

Start all services:
\`\`\`bash
./quotron start
\`\`\`

Start specific services:
\`\`\`bash
./quotron start api dashboard
\`\`\`

Stop all services:
\`\`\`bash
./quotron stop
\`\`\`

Check service status:
\`\`\`bash
./quotron status
\`\`\`

Check health of services:
\`\`\`bash
./quotron health
./quotron health --action system
./quotron health --action service api-scraper/yahoo_finance
./quotron health --format json
\`\`\`

Run tests:
\`\`\`bash
./quotron test
./quotron test api
./quotron test job stock_quotes
\`\`\`

Import S&P 500 data:
\`\`\`bash
./quotron import-sp500
\`\`\`

Generate a default config file:
\`\`\`bash
./quotron --gen-config
\`\`\`

Start services in monitor mode (auto-restart):
\`\`\`bash
./quotron start --monitor
\`\`\`

## Configuration

The CLI can be configured via:

1. Default values
2. Environment variables 
3. Config file (JSON format)
4. Command-line flags

To generate a default config file:
\`\`\`bash
./quotron --gen-config
\`\`\`

This will create a \`quotron.json\` file that you can edit to customize the configuration.

## Services

The CLI manages the following services:

- **YFinance Proxy**: Python service that interfaces with Yahoo Finance API
- **API Service**: Go service that provides REST API endpoints with smart failover between data sources
- **Scheduler**: Go service that schedules data collection jobs
- **Dashboard**: Python service that provides a web UI
- **Health Service**: Go service that monitors and reports on system health

The services have the following dependencies:
- API Service requires YFinance Proxy (automatically starts it if needed)
- Dashboard typically requires API Service (automatically starts it if needed)
- Scheduler can work with either API Service or direct data sources 

When starting services with \`quotron start\`, the CLI automatically resolves these dependencies, ensuring proper service startup order.

## Development

To extend the CLI with new functionality:

1. Implement a new command that implements the \`Command\` interface in \`pkg/services\`
2. Register the command in \`getAvailableCommands()\` in \`cmd/main/main.go\`
3. Update the help text in the \`usage()\` function
4. Run \`go mod tidy\` to update dependencies
5. Build with \`./build.sh\`
EOF

  # Replace the original README with the updated one
  mv README.md.new README.md
  echo "README.md updated with current CLI help and commit hash ${COMMIT_HASH}"
  fi
fi

echo "Build completed successfully!"
echo "You can run the CLI with: ./quotron [command] [options]"
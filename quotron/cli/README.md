# Quotron CLI

[![CLI Build:838ee76](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/cli-release.yml?label=CLI%20Build%3A838ee76&logo=go)](https://github.com/we-be/tiny-ria/actions/workflows/cli-release.yml)

This CLI provides a unified interface for managing all Quotron services and operations.

## Overview

The Quotron CLI replaces the various bash scripts that were previously used to manage the Quotron system, consolidating them into a single, consistent interface. It provides commands for starting and stopping services, running tests, and importing data.

## Installation

To build and install the CLI, run:

```bash
cd cli
./build.sh
```

This will create a `quotron` binary in the parent directory.

## Usage

<!-- CLI_HELP_START -->
```
Quotron - Financial data system CLI

Usage: quotron [OPTIONS] COMMAND [ARGS]

Options:
  --config FILE      Path to config file
  --log-level LEVEL  Set log level (debug, info, warn, error)
  --force            Force operations even if conflicts detected
  --gen-config       Generate default config file
  --monitor          Monitor services and restart if they fail

Commands:
  start [SERVICE...]  Start services (all or specified services)
  stop [SERVICE...]   Stop services (all or specified services)
  status              Show status of all services
  test [TEST]         Run tests (all or specified test)
  import-sp500        Import S&P 500 data
  scheduler <SUBCOMMAND>  Manage or interact with the scheduler:
                       - jobs: List configured jobs
                       - run-job <JOBNAME>: Run a job immediately
                       - next-runs: Show upcoming execution times
                       - status: Show scheduler status
                       - help: Show detailed scheduler help
  health              Check health of services
  help                Show this help message

Services:
  all                 All services (default)
  proxy               YFinance proxy only
  api                 API service only
  dashboard           Dashboard only
  scheduler           Scheduler only
  etl                 ETL service only

Tests:
  all                 All tests (default)
  api                 API service tests
  integration         Integration tests
  job JOBNAME         Run a specific job test
```
<!-- CLI_HELP_END -->

## Examples

Start all services:
```bash
./quotron start
```

Start specific services:
```bash
./quotron start api dashboard
```

Stop all services:
```bash
./quotron stop
```

Check service status:
```bash
./quotron status
```

Check health of services:
```bash
./quotron health
./quotron health --action system
./quotron health --action service api-scraper/yahoo_finance
./quotron health --format json
```

Run tests:
```bash
./quotron test
./quotron test api
./quotron test job stock_quotes
```

Import S&P 500 data:
```bash
./quotron import-sp500
```

Generate a default config file:
```bash
./quotron --gen-config
```

Start services in monitor mode (auto-restart):
```bash
./quotron start --monitor
```

## Configuration

The CLI can be configured via:

1. Default values
2. Environment variables 
3. Config file (JSON format)
4. Command-line flags

To generate a default config file:
```bash
./quotron --gen-config
```

This will create a `quotron.json` file that you can edit to customize the configuration.

## Services

The CLI manages the following services:

- **YFinance Proxy**: Python service that interfaces with Yahoo Finance API
- **API Service**: Go service that provides REST API endpoints with smart failover between data sources
- **Scheduler**: Go service that schedules data collection jobs
- **Dashboard**: Python service that provides a web UI
- **ETL Service**: Go service that processes data from Redis streams and stores it in the database
- **Health Service**: Go service that monitors and reports on system health

The services have the following dependencies:
- API Service requires YFinance Proxy (automatically starts it if needed)
- Dashboard typically requires API Service (automatically starts it if needed)
- Scheduler can work with either API Service or direct data sources
- ETL Service requires Redis and PostgreSQL database access

When starting services with `quotron start`, the CLI automatically resolves these dependencies, ensuring proper service startup order.

## Development

To extend the CLI with new functionality:

1. Implement a new command that implements the `Command` interface in `pkg/services`
2. Register the command in `getAvailableCommands()` in `cmd/main/main.go`
3. Update the help text in the `usage()` function
4. Run `go mod tidy` to update dependencies
5. Build with `./build.sh`

# Quotron CLI

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
  help                Show this help message

Services:
  all                 All services (default)
  proxy               YFinance proxy only
  api                 API service only
  dashboard           Dashboard only
  scheduler           Scheduler only

Tests:
  all                 All tests (default)
  api                 API service tests
  integration         Integration tests
  job JOBNAME         Run a specific job test
```

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

- **YFinance Proxy**: Python service that interfaces with Yahoo Finance
- **API Service**: Go service that provides REST API endpoints
- **Scheduler**: Go service that schedules data collection jobs
- **Dashboard**: Python service that provides a web UI

## Development

To extend the CLI with new functionality:

1. Add new command handlers in `cmd/main/main.go`
2. Implement service logic in the `pkg/services` package
3. Run `go mod tidy` to update dependencies
4. Build with `./build.sh`
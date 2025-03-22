# tiny-ria

A modular financial data scraping, analysis, and trading system.

[![CLI:312becf](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/cli-release.yml?label=CLI%3A312becf&logo=go)](https://github.com/we-be/tiny-ria/actions/workflows/cli-release.yml)
[![YFinance](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/yahoo-finance-tests.yml?label=YFinance&logo=yahoo)](https://github.com/we-be/tiny-ria/actions/workflows/yahoo-finance-tests.yml)
[![API Scraper](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/api-scraper-tests.yml?label=API%20Scraper&logo=golang)](https://github.com/we-be/tiny-ria/actions/workflows/api-scraper-tests.yml)

## Quick Start

```bash
# Clone the repo
git clone https://github.com/we-be/tiny-ria.git
cd tiny-ria

# Download the latest CLI release
curl -L -o quotron "$(curl -s https://api.github.com/repos/we-be/tiny-ria/releases/latest | grep -o 'https://github.com/we-be/tiny-ria/releases/download/[^/]*/.*linux')"
chmod +x quotron

# Start the YFinance proxy (no API key required)
./quotron start yfinance-proxy

# Fetch stock data
cd quotron/api-scraper
go build -o api-scraper ./cmd/main/main.go
./api-scraper --yahoo --symbol AAPL
```

## Architecture

**Quotron** is a modular financial data system with the following key components:

- **CLI** (Go): Unified interface for managing all services and operations
- **API Scraper** (Go): Fetches data from financial APIs with automatic failover
  - Alpha Vantage provider for authentic market data
  - Yahoo Finance provider as a free alternative with higher rate limits
- **Browser Scraper** (Python/Playwright): Extracts data from JS-heavy websites
- **API Service** (Go): Middleware layer that provides a standardized API interface and dashboard UI
- **Storage** (PostgreSQL): Persists financial data for historical analysis
- **Scheduler** (Go): Orchestrates recurring data collection tasks
- **Health** (Go): Monitors and reports on the health of all services

The system is designed with resilience in mind - components can operate independently and gracefully degrade when dependencies are unavailable.

### Data Flow

1. **Collection**: CLI triggers data collection via Scheduler or direct commands
2. **Acquisition**: API Scraper fetches financial data with automatic source failover
3. **Processing**: Data undergoes validation, normalization, and enrichment
4. **Storage**: Processed data is persisted to PostgreSQL for historical analysis
5. **Access**: Applications retrieve data through the API Service's unified interface
6. **Monitoring**: Health component continuously tracks service status and data quality

## CLI Reference

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
                       - run-job <JOBNAME>: Run a job immediately
                       - crypto_quotes: Fetch cryptocurrency quotes
                       - status: Show scheduler status
                       - help: Show detailed scheduler help
  health              Check health of services
  help                Show this help message

Services:
  all                 All services (default)
  proxy               YFinance proxy only
  api                 API service only
  # dashboard service has been integrated into the API service
  scheduler           Scheduler only
  etl                 ETL service only

Tests:
  all                 All tests (default)
  api                 API service tests
  integration         Integration tests
  job JOBNAME         Run a specific job test
```
<!-- CLI_HELP_END -->

## Development

### API Keys

To run the API scrapers with real data, you'll need:
- Alpha Vantage API key (set as `ALPHA_VANTAGE_API_KEY` environment variable)

### GitHub Actions

The repo includes several CI workflows:

1. **CLI Release Workflow**: Builds and publishes the Quotron CLI as a GitHub release
2. **Yahoo Finance Tests**: Verifies the Yahoo Finance proxy and direct API integration
3. **API Scraper Tests**: Tests the API scraper with Alpha Vantage and integration tests

Status badges at the top of this README show the current health of these workflows.

#### Setting Up API Keys

To enable API tests in GitHub Actions, add your Alpha Vantage API key as a repository secret:
- Go to Settings → Secrets → Actions → New repository secret
- Name: `ALPHA_VANTAGE_API_KEY`
- Value: Your API key

#### Using the CLI

The Quotron CLI binary is published as a GitHub release and can be used to manage all services:

```bash
# Start services
./quotron start yfinance-proxy
./quotron start api-service

# Check service status
./quotron status

# Stop services
./quotron stop yfinance-proxy
./quotron stop api-service

# Check health
./quotron health
```

---
Darvas

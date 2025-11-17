# tiny-ria

A modular financial data platform with AI assistant capabilities for analysis and alerts.

[![CLI:b10c5d8](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/cli-release.yml?label=CLI%3Ab10c5d8&logo=go)](https://github.com/we-be/tiny-ria/actions/workflows/cli-release.yml)
[![YFinance](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/yahoo-finance-tests.yml?label=YFinance&logo=yahoo)](https://github.com/we-be/tiny-ria/actions/workflows/yahoo-finance-tests.yml)
[![API Scraper](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/api-scraper-tests.yml?label=API%20Scraper&logo=golang)](https://github.com/we-be/tiny-ria/actions/workflows/api-scraper-tests.yml)

## Agent Features

The **Agent** module provides intelligent financial monitoring and assistance:

- **Chat UI**: Interactive interface for natural language conversations about your financial data
- **AI Assistant**: Answers questions, provides market insights, and helps with financial decision-making
- **Alerting System**: Monitors price movements and sends intelligent alerts
- **Real-time Data**: Connects to live financial data sources for up-to-date information
- **Portfolio Analysis**: Tracks and analyzes your investments

### Quick Start - Agent

```bash
# Clone the repo
git clone https://github.com/we-be/tiny-ria.git
cd tiny-ria/agent

# Build the agent binaries
./build.sh

# Start the unified UI (chat and dashboard)
./bin/unified

# Start just the chat UI
./bin/chat-ui

# Start the AI assistant service
./bin/assistant

# Start the AI alerter
./bin/ai-alerter
```

Access the UI at http://localhost:8080

## Quotron - Financial Data Backend

The **Quotron** module handles data collection and processing:

```bash
# Start the services
cd quotron
./quotron start yfinance-proxy
./quotron start api-service

# Check health
./quotron health

# Fetch stock data
./quotron scheduler run-job stock_quotes --symbols AAPL,MSFT,GOOG
```

## Architecture

- **Agent**: AI-powered assistant and alerting system
- **Quotron**: Financial data collection and processing pipeline
- **MCP**: Master Control Program for system orchestration

See `agent/README.md` and `quotron/README.md` for more detailed documentation.

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

### Requirements

- Go 1.18+
- Python 3.9+
- PostgreSQL 13+
- Redis 6+

### API Keys

For full functionality, you'll need:
- Alpha Vantage API key (set as `ALPHA_VANTAGE_API_KEY` environment variable)

Free alternative:
- Yahoo Finance data (no API key required)

---
